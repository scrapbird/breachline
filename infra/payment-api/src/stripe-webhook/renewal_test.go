package main

import (
	"testing"
	"time"

	"github.com/stripe/stripe-go/v78"
)

func TestLicenseDaysUntil(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		end  time.Time
		want int
	}{
		{"monthly period", now.AddDate(0, 1, 0), 31 + licenseGraceDays},
		{"yearly period", now.AddDate(1, 0, 0), 365 + licenseGraceDays},
		{"partial day rounds up", now.Add(36 * time.Hour), 2 + licenseGraceDays},
		{"period already over", now.Add(-time.Hour), 1 + licenseGraceDays},
	}
	for _, tc := range cases {
		if got := licenseDaysUntil(tc.end.Unix(), now); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func invoice(reason stripe.InvoiceBillingReason, lines ...*stripe.InvoiceLineItem) *stripe.Invoice {
	return &stripe.Invoice{BillingReason: reason, Lines: &stripe.InvoiceLineItemList{Data: lines}}
}

func line(product string, end int64) *stripe.InvoiceLineItem {
	return &stripe.InvoiceLineItem{Price: price(product, nil), Period: &stripe.Period{End: end}}
}

func TestIsPremiumRenewal(t *testing.T) {
	cases := []struct {
		name string
		inv  *stripe.Invoice
		want bool
	}{
		{"premium cycle", invoice(stripe.InvoiceBillingReasonSubscriptionCycle, line(productPremium, 100)), true},
		{"premium plan switch", invoice(stripe.InvoiceBillingReasonSubscriptionUpdate, line(productPremium, 100)), true},
		{"first invoice (handled by checkout)", invoice(stripe.InvoiceBillingReasonSubscriptionCreate, line(productPremium, 100)), false},
		{"sync-only cycle", invoice(stripe.InvoiceBillingReasonSubscriptionCycle, line(productSyncSeats, 100)), false},
		{"manual invoice", invoice(stripe.InvoiceBillingReasonManual, line(productPremium, 100)), false},
		{"no lines", &stripe.Invoice{BillingReason: stripe.InvoiceBillingReasonSubscriptionCycle}, false},
	}
	for _, tc := range cases {
		if got := isPremiumRenewal(tc.inv); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPremiumPeriodEndTakesLatest(t *testing.T) {
	inv := invoice(stripe.InvoiceBillingReasonSubscriptionCycle,
		line(productPremium, 100), line(productSyncSeats, 999), line(productPremium, 200))
	if end, ok := premiumPeriodEnd(inv); !ok || end != 200 {
		t.Errorf("got %d %v, want 200 true", end, ok)
	}
}
