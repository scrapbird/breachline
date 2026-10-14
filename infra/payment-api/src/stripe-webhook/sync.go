package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/checkout/session"
	"github.com/stripe/stripe-go/v78/customer"
	"github.com/stripe/stripe-go/v78/subscription"
)

// Values of the breachline_product metadata key that infra/stripe sets on
// every product and price, so purchases are classified without hardcoding
// Stripe IDs (which differ between the sandbox and live accounts).
const (
	productPremium        = "premium"
	productSyncWorkspaces = "sync_workspaces"
	productSyncSeats      = "sync_seats"

	// Workspaces per sync pack when the price carries no workspaces_per_unit.
	defaultWorkspacesPerPack = 5
)

var (
	ddbClient              *dynamodb.Client
	userSubscriptionsTable string
)

// purchaseKind summarises which BreachLine products a checkout contains.
type purchaseKind struct {
	premium bool
	sync    bool
}

// SyncEntitlements are the sync-api limits for one email address.
type SyncEntitlements struct {
	WorkspaceLimit int
	SeatCount      int
}

// productOf returns the breachline_product of a price, or "" if unknown.
func productOf(price *stripe.Price) string {
	if price == nil {
		return ""
	}
	return price.Metadata["breachline_product"]
}

// isSyncProduct reports whether product is a workspace pack or a seat.
func isSyncProduct(product string) bool {
	return product == productSyncWorkspaces || product == productSyncSeats
}

// classifyCheckout lists the session's line items and reports which
// BreachLine products were bought.
func classifyCheckout(sessionID string) (purchaseKind, error) {
	var kind purchaseKind
	params := &stripe.CheckoutSessionListLineItemsParams{Session: stripe.String(sessionID)}
	iter := session.ListLineItems(params)
	for iter.Next() {
		item := iter.LineItem()
		switch product := productOf(item.Price); {
		case product == productPremium:
			kind.premium = true
		case isSyncProduct(product):
			kind.sync = true
		default:
			priceID := ""
			if item.Price != nil {
				priceID = item.Price.ID
			}
			logger.Warn("Checkout line item is not a BreachLine product",
				"session_id", sessionID,
				"price_id", priceID)
		}
	}
	if err := iter.Err(); err != nil {
		return kind, fmt.Errorf("listing line items: %w", err)
	}
	return kind, nil
}

// subscriptionGrantsAccess reports whether a subscription in this status
// should keep its sync entitlements. past_due keeps access while Stripe
// retries the payment; unpaid/canceled/incomplete do not.
func subscriptionGrantsAccess(status stripe.SubscriptionStatus) bool {
	switch status {
	case stripe.SubscriptionStatusActive,
		stripe.SubscriptionStatusTrialing,
		stripe.SubscriptionStatusPastDue:
		return true
	}
	return false
}

// entitlementsFromSubscriptions sums the sync packs and seats across every
// subscription that still grants access.
func entitlementsFromSubscriptions(subs []*stripe.Subscription) SyncEntitlements {
	var ent SyncEntitlements
	for _, sub := range subs {
		if sub == nil || !subscriptionGrantsAccess(sub.Status) || sub.Items == nil {
			continue
		}
		for _, item := range sub.Items.Data {
			if item == nil {
				continue
			}
			switch productOf(item.Price) {
			case productSyncWorkspaces:
				perPack := defaultWorkspacesPerPack
				if v, err := strconv.Atoi(item.Price.Metadata["workspaces_per_unit"]); err == nil && v > 0 {
					perPack = v
				}
				ent.WorkspaceLimit += int(item.Quantity) * perPack
			case productSyncSeats:
				ent.SeatCount += int(item.Quantity)
			}
		}
	}
	return ent
}

// subscriptionHasSync reports whether a subscription contains any sync item.
func subscriptionHasSync(sub *stripe.Subscription) bool {
	if sub == nil || sub.Items == nil {
		return false
	}
	for _, item := range sub.Items.Data {
		if item != nil && isSyncProduct(productOf(item.Price)) {
			return true
		}
	}
	return false
}

// subscriptionsForEmail lists every subscription of every Stripe customer
// with this email. Payment links create a new customer per checkout, so one
// person buying packs and seats separately ends up with several customers.
func subscriptionsForEmail(email string) ([]*stripe.Subscription, error) {
	var subs []*stripe.Subscription
	customers := customer.List(&stripe.CustomerListParams{Email: stripe.String(email)})
	for customers.Next() {
		cust := customers.Customer()
		iter := subscription.List(&stripe.SubscriptionListParams{
			Customer: stripe.String(cust.ID),
			Status:   stripe.String("all"),
		})
		for iter.Next() {
			subs = append(subs, iter.Subscription())
		}
		if err := iter.Err(); err != nil {
			return nil, fmt.Errorf("listing subscriptions for customer %s: %w", cust.ID, err)
		}
	}
	if err := customers.Err(); err != nil {
		return nil, fmt.Errorf("listing customers: %w", err)
	}
	return subs, nil
}

// reconcileSyncEntitlements recomputes an email's sync limits from Stripe
// and writes them to the sync-api user_subscriptions table. Deriving the
// totals from Stripe on every event (instead of applying deltas) makes it
// idempotent and safe against retried or out-of-order webhooks.
func reconcileSyncEntitlements(ctx context.Context, email string) error {
	if userSubscriptionsTable == "" {
		return fmt.Errorf("USER_SUBSCRIPTIONS_TABLE not set")
	}
	subs, err := subscriptionsForEmail(email)
	if err != nil {
		return err
	}
	ent := entitlementsFromSubscriptions(subs)
	logger.Info("Reconciled sync entitlements from Stripe",
		"email", email,
		"subscriptions", len(subs),
		"workspace_limit", ent.WorkspaceLimit,
		"seat_count", ent.SeatCount)

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(userSubscriptionsTable),
		Key: map[string]types.AttributeValue{
			"email": &types.AttributeValueMemberS{Value: email},
		},
		UpdateExpression: aws.String("SET workspace_limit = :wl, seat_count = :sc, updated_at = :now, " +
			"created_at = if_not_exists(created_at, :now)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":wl":  &types.AttributeValueMemberN{Value: strconv.Itoa(ent.WorkspaceLimit)},
			":sc":  &types.AttributeValueMemberN{Value: strconv.Itoa(ent.SeatCount)},
			":now": &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return fmt.Errorf("updating %s: %w", userSubscriptionsTable, err)
	}
	return nil
}

// handleSubscriptionEvent reconciles sync entitlements when a subscription
// containing sync items is created, changed (e.g. seats added in the
// portal) or cancelled.
func handleSubscriptionEvent(ctx context.Context, sub *stripe.Subscription) error {
	if !subscriptionHasSync(sub) {
		logger.Info("Subscription has no sync items, ignoring", "subscription_id", sub.ID)
		return nil
	}
	if sub.Customer == nil || sub.Customer.ID == "" {
		return fmt.Errorf("subscription %s has no customer", sub.ID)
	}
	cust, err := customer.Get(sub.Customer.ID, nil)
	if err != nil {
		return fmt.Errorf("fetching customer %s: %w", sub.Customer.ID, err)
	}
	if cust.Email == "" {
		return fmt.Errorf("customer %s has no email", cust.ID)
	}
	return reconcileSyncEntitlements(ctx, cust.Email)
}
