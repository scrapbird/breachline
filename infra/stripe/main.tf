# Stripe catalogue + webhook for one environment. Pricing (doc/PRICING.md):
#   Premium license       10 USD / month, or 100 USD / year
#   Workspace sync        5 USD / month per pack of 5 workspaces (base plan = 1 pack)
#   Shared seats          1 USD / month per seat (1 member in 1 workspace)
#
# lookup_keys and metadata identify each price to code (e.g. the webhook can
# tell a premium purchase from a sync purchase) without hardcoding Stripe IDs,
# which differ between the sandbox and live accounts.

locals {
  # The payment-api stack owns the webhook Lambda + the (empty) Secrets
  # Manager secret; this stack points Stripe at it and fills the secret in.
  payment_api = data.terraform_remote_state.payment_api.outputs
}

data "terraform_remote_state" "payment_api" {
  backend = "s3"

  config = {
    bucket = "breachline-state-uf308ht4"
    key    = "payment-api/${var.environment}/terraform.tfstate"
    region = "us-east-2"
  }
}

#================================================
# Premium license
#================================================

resource "stripe_product" "premium" {
  name                 = "BreachLine Premium"
  description          = "Premium license for the BreachLine desktop app: workspaces, row annotations and exports."
  statement_descriptor = "BREACHLINE PREMIUM"

  metadata = {
    breachline_product = "premium"
  }
}

resource "stripe_price" "premium_monthly" {
  product     = stripe_product.premium.id
  currency    = "usd"
  unit_amount = 1000
  lookup_key  = "premium_monthly"
  nickname    = "Premium monthly"

  recurring {
    interval = "month"
  }

  metadata = {
    breachline_product = "premium"
  }
}

resource "stripe_price" "premium_yearly" {
  product     = stripe_product.premium.id
  currency    = "usd"
  unit_amount = 10000
  lookup_key  = "premium_yearly"
  nickname    = "Premium yearly"

  recurring {
    interval = "year"
  }

  metadata = {
    breachline_product = "premium"
  }
}

#================================================
# Workspace sync service
#================================================

# Sold by quantity: 1 pack = 5 online workspaces (workspace_limit = 5 x qty).
resource "stripe_product" "sync_workspaces" {
  name                 = "BreachLine Workspace Sync"
  description          = "Cloud sync for BreachLine workspaces, sold in packs of 5 online workspaces."
  statement_descriptor = "BREACHLINE SYNC"
  unit_label           = "pack" # of 5 workspaces (max 12 chars)

  metadata = {
    breachline_product  = "sync_workspaces"
    workspaces_per_unit = "5"
  }
}

resource "stripe_price" "sync_workspace_pack" {
  product     = stripe_product.sync_workspaces.id
  currency    = "usd"
  unit_amount = 500
  lookup_key  = "sync_workspace_pack_monthly"
  nickname    = "Workspace sync pack (5 workspaces) monthly"

  recurring {
    interval = "month"
  }

  metadata = {
    breachline_product  = "sync_workspaces"
    workspaces_per_unit = "5"
  }
}

# Sold by quantity: 1 seat = 1 team member in 1 shared workspace.
resource "stripe_product" "sync_seats" {
  name                 = "BreachLine Shared Seats"
  description          = "Seats for sharing synced workspaces: each seat adds one team member to one workspace."
  statement_descriptor = "BREACHLINE SEATS"
  unit_label           = "seat"

  metadata = {
    breachline_product = "sync_seats"
  }
}

resource "stripe_price" "sync_seat" {
  product     = stripe_product.sync_seats.id
  currency    = "usd"
  unit_amount = 100
  lookup_key  = "sync_seat_monthly"
  nickname    = "Shared seat monthly"

  recurring {
    interval = "month"
  }

  metadata = {
    breachline_product = "sync_seats"
  }
}

#================================================
# Payment links
#================================================

# The stripe-webhook Lambda classifies each checkout by its prices'
# breachline_product metadata: premium issues a license; sync packs and
# seats are summed across the buyer's active subscriptions into sync-api's
# user_subscriptions (workspace_limit = 5 x packs, seat_count = seats).

locals {
  after_completion = var.success_url != "" ? {
    type     = "redirect"
    redirect = { url = var.success_url }
    } : {
    type     = "hosted_confirmation"
    redirect = null
  }

  premium_links = {
    monthly = stripe_price.premium_monthly.id
    yearly  = stripe_price.premium_yearly.id
  }
}

resource "stripe_payment_link" "premium" {
  for_each = local.premium_links

  line_items = [
    {
      price    = each.value
      quantity = 1
    }
  ]

  after_completion = {
    type     = local.after_completion.type
    redirect = local.after_completion.redirect
  }

  allow_promotion_codes = true

  metadata = {
    breachline_product = "premium"
    plan               = each.key
  }
}

# Workspace sync: packs of 5 workspaces (at least 1, the base plan), with
# shared seats as an optional add-on in the same checkout.
resource "stripe_payment_link" "sync" {
  line_items = [
    {
      price    = stripe_price.sync_workspace_pack.id
      quantity = 1
      adjustable_quantity = {
        enabled = true
        minimum = 1
        maximum = 100
      }
    }
  ]

  optional_items = [
    {
      price    = stripe_price.sync_seat.id
      quantity = 1
      adjustable_quantity = {
        enabled = true
        minimum = 1
        maximum = 500
      }
    }
  ]

  after_completion = {
    type     = local.after_completion.type
    redirect = local.after_completion.redirect
  }

  allow_promotion_codes = true

  metadata = {
    breachline_product = "sync_workspaces"
    plan               = "sync"
  }
}

# Extra seats on their own, for existing sync customers sharing more
# workspaces. Each purchase is a separate subscription; the webhook sums them.
resource "stripe_payment_link" "seats" {
  line_items = [
    {
      price    = stripe_price.sync_seat.id
      quantity = 1
      adjustable_quantity = {
        enabled = true
        minimum = 1
        maximum = 500
      }
    }
  ]

  after_completion = {
    type     = local.after_completion.type
    redirect = local.after_completion.redirect
  }

  allow_promotion_codes = true

  metadata = {
    breachline_product = "sync_seats"
    plan               = "seats"
  }
}

#================================================
# Webhook endpoint -> payment-api
#================================================

resource "stripe_webhook_endpoint" "payment_api" {
  url = local.payment_api.webhook_url
  # checkout: first premium license + new sync purchases; invoice.paid:
  # renewed premium licenses; subscription events: keep sync limits in step
  # with quantity changes and cancellations.
  enabled_events = [
    "checkout.session.completed",
    "invoice.paid",
    "customer.subscription.created",
    "customer.subscription.updated",
    "customer.subscription.deleted",
  ]
  api_version = var.webhook_api_version
  description = "BreachLine payment-api (${var.environment})"
}

# Stripe only returns the signing secret when the endpoint is created, so it
# is written straight into the secret payment-api's Lambda reads. Replacing the
# endpoint rotates it; the Lambda reloads on the next failed verification.
resource "aws_secretsmanager_secret_version" "webhook_secret" {
  secret_id     = local.payment_api.stripe_webhook_secret_arn
  secret_string = stripe_webhook_endpoint.payment_api.secret
}
