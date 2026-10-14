package main

import (
	"fmt"
	"math"
	"time"

	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/customer"
)

const (
	// Licenses outlive the paid period by this much, so a renewal that is
	// late (Stripe retrying a failed payment, a delayed webhook, the customer
	// importing the new file) doesn't lock a paying customer out.
	licenseGraceDays = 7

	// Used when the subscription period can't be read.
	defaultLicenseDays = 365
)

// licenseDaysUntil returns how many days a license must be valid for to
// cover a billing period ending at periodEnd (unix seconds), plus the grace
// window. Partial days round up.
func licenseDaysUntil(periodEnd int64, now time.Time) int {
	days := int(math.Ceil(time.Unix(periodEnd, 0).Sub(now).Hours() / 24))
	if days < 1 {
		days = 1
	}
	return days + licenseGraceDays
}

// isPremiumRenewal reports whether a paid invoice extends an existing
// premium subscription. subscription_create is skipped: the first license is
// issued from checkout.session.completed. subscription_update covers plan
// switches (e.g. monthly to yearly) that start a new period.
func isPremiumRenewal(inv *stripe.Invoice) bool {
	switch inv.BillingReason {
	case stripe.InvoiceBillingReasonSubscriptionCycle, stripe.InvoiceBillingReasonSubscriptionUpdate:
	default:
		return false
	}
	_, ok := premiumPeriodEnd(inv)
	return ok
}

// premiumPeriodEnd returns the latest period end of the invoice's premium
// line items.
func premiumPeriodEnd(inv *stripe.Invoice) (int64, bool) {
	if inv.Lines == nil {
		return 0, false
	}
	var end int64
	for _, line := range inv.Lines.Data {
		if line == nil || productOf(line.Price) != productPremium || line.Period == nil {
			continue
		}
		if line.Period.End > end {
			end = line.Period.End
		}
	}
	return end, end > 0
}

// handleInvoicePaid issues a renewed premium license, valid to the end of
// the newly paid period, when a premium subscription renews.
func handleInvoicePaid(inv *stripe.Invoice) error {
	if !isPremiumRenewal(inv) {
		logger.Info("Invoice is not a premium renewal, ignoring",
			"invoice_id", inv.ID,
			"billing_reason", inv.BillingReason)
		return nil
	}
	periodEnd, _ := premiumPeriodEnd(inv)

	email := inv.CustomerEmail
	if email == "" && inv.Customer != nil && inv.Customer.ID != "" {
		cust, err := customer.Get(inv.Customer.ID, nil)
		if err != nil {
			return fmt.Errorf("fetching customer %s: %w", inv.Customer.ID, err)
		}
		email = cust.Email
	}
	if email == "" {
		return fmt.Errorf("invoice %s has no customer email", inv.ID)
	}

	days := licenseDaysUntil(periodEnd, time.Now())
	logger.Info("Premium subscription renewed, issuing license",
		"invoice_id", inv.ID,
		"billing_reason", inv.BillingReason,
		"period_end", periodEnd,
		"license_days", days)
	if ok, errMsg := publishLicenseRequest(email, days, inv.ID); !ok {
		return fmt.Errorf("publishing license request: %s", errMsg)
	}
	return nil
}
