package main

import (
	"testing"

	"github.com/stripe/stripe-go/v78"
)

func price(product string, meta map[string]string) *stripe.Price {
	m := map[string]string{"breachline_product": product}
	for k, v := range meta {
		m[k] = v
	}
	return &stripe.Price{Metadata: m}
}

func sub(status stripe.SubscriptionStatus, items ...*stripe.SubscriptionItem) *stripe.Subscription {
	return &stripe.Subscription{Status: status, Items: &stripe.SubscriptionItemList{Data: items}}
}

func item(p *stripe.Price, qty int64) *stripe.SubscriptionItem {
	return &stripe.SubscriptionItem{Price: p, Quantity: qty}
}

func TestEntitlementsFromSubscriptions(t *testing.T) {
	pack := price(productSyncWorkspaces, map[string]string{"workspaces_per_unit": "5"})
	packNoMeta := price(productSyncWorkspaces, nil)
	seat := price(productSyncSeats, nil)
	premium := price(productPremium, nil)

	cases := []struct {
		name string
		subs []*stripe.Subscription
		want SyncEntitlements
	}{
		{"nothing", nil, SyncEntitlements{}},
		{"base plan", []*stripe.Subscription{sub(stripe.SubscriptionStatusActive, item(pack, 1))}, SyncEntitlements{5, 0}},
		{"packs + seats in one checkout", []*stripe.Subscription{
			sub(stripe.SubscriptionStatusActive, item(pack, 2), item(seat, 3)),
		}, SyncEntitlements{10, 3}},
		{"separate purchases are summed", []*stripe.Subscription{
			sub(stripe.SubscriptionStatusActive, item(pack, 1)),
			sub(stripe.SubscriptionStatusTrialing, item(seat, 2)),
			sub(stripe.SubscriptionStatusPastDue, item(packNoMeta, 1)),
		}, SyncEntitlements{10, 2}},
		{"cancelled and unpaid grant nothing", []*stripe.Subscription{
			sub(stripe.SubscriptionStatusCanceled, item(pack, 4)),
			sub(stripe.SubscriptionStatusUnpaid, item(seat, 4)),
			sub(stripe.SubscriptionStatusIncomplete, item(seat, 4)),
		}, SyncEntitlements{}},
		{"premium does not count", []*stripe.Subscription{
			sub(stripe.SubscriptionStatusActive, item(premium, 1), item(seat, 1)),
		}, SyncEntitlements{0, 1}},
	}
	for _, tc := range cases {
		if got := entitlementsFromSubscriptions(tc.subs); got != tc.want {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestSubscriptionHasSync(t *testing.T) {
	if subscriptionHasSync(sub(stripe.SubscriptionStatusActive, item(price(productPremium, nil), 1))) {
		t.Error("premium-only subscription reported as sync")
	}
	if !subscriptionHasSync(sub(stripe.SubscriptionStatusCanceled, item(price(productSyncSeats, nil), 1))) {
		t.Error("cancelled seat subscription should still be recognised as sync")
	}
	if subscriptionHasSync(nil) {
		t.Error("nil subscription reported as sync")
	}
}
