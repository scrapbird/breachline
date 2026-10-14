output "payment_links" {
  description = "Public payment link URLs by premium plan (consumed by the website build)"
  value       = { for plan, link in stripe_payment_link.premium : plan => link.url }
}

output "price_ids" {
  description = "Stripe price IDs by lookup key"
  value = {
    (stripe_price.premium_monthly.lookup_key)     = stripe_price.premium_monthly.id
    (stripe_price.premium_yearly.lookup_key)      = stripe_price.premium_yearly.id
    (stripe_price.sync_workspace_pack.lookup_key) = stripe_price.sync_workspace_pack.id
    (stripe_price.sync_seat.lookup_key)           = stripe_price.sync_seat.id
  }
}

output "webhook_endpoint_id" {
  description = "Stripe webhook endpoint pointed at payment-api"
  value       = stripe_webhook_endpoint.payment_api.id
}
