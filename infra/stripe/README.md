# BreachLine Stripe Configuration

This Terraform stack manages the Stripe side of BreachLine for one environment: the product catalogue, the premium payment links used by the website, and the webhook endpoint that drives the payment-api. `dev` runs against the Stripe sandbox and `prod` against the live account, selected by which API key is in the vault.

It uses Stripe's official Terraform provider (`stripe/stripe`), pinned exactly to `0.3.0` because the provider is pre-1.0. Review its changelog before bumping.

## What it creates

| Resource | Details |
|---|---|
| Product: BreachLine Premium | Prices: 10 USD / month (`premium_monthly`), 100 USD / year (`premium_yearly`) |
| Product: BreachLine Workspace Sync | Price: 5 USD / month per pack of 5 workspaces (`sync_workspace_pack_monthly`), sold by quantity |
| Product: BreachLine Shared Seats | Price: 1 USD / month per seat (`sync_seat_monthly`), sold by quantity |
| Payment links | Premium monthly and yearly, with promotion codes allowed |
| Webhook endpoint | `checkout.session.completed` to payment-api's `webhook_url`, API version `2024-04-10` |
| Secrets Manager secret version | The webhook signing secret, written into payment-api's `breachline-stripe-webhook-secret-<env>` |

Pricing follows `doc/PRICING.md`. Each price has a `lookup_key` and `breachline_product` metadata so code can identify it without hardcoding Stripe IDs, which differ between the sandbox and live accounts.

There are no payment links for workspace sync or seats yet: the webhook Lambda treats every completed checkout as a premium purchase, so selling sync before it can provision `user_subscriptions` would issue a license instead (see `doc/TODO.md`).

## Dependencies

- `infra/payment-api` for the same environment must be applied first. This stack reads its `webhook_url` and `stripe_webhook_secret_arn` outputs from remote state.
- The website build reads this stack's `payment_links` output (see `make build-website`), so apply this before the website.

`make deploy ENV=<env>` runs everything in the right order.

## Input variables

| Variable | Description | Source |
|---|---|---|
| `environment` | `dev` or `prod`; selects the payment-api state to read | `make render` |
| `aws_region` | Region of the payment-api stack | `config.aws.region` |
| `stripe_api_key` | Stripe secret key (`sk_test_` for dev, `sk_live_` for prod), sensitive | `vault.environments.<env>.stripe.api_key` as `TF_VAR_stripe_api_key` |
| `success_url` | Where payment links redirect after checkout; empty uses Stripe's hosted confirmation page | `config.environments.<env>.stripe.success_url` |
| `webhook_api_version` | API version webhook events are rendered in; must match payment-api's stripe-go major version | default `2024-04-10` (stripe-go v78) |

`make render` writes `infra/stripe/<env>.tfvars` (gitignored) and `secrets.env`.

## State

State is stored at `stripe/<env>/terraform.tfstate` in the `breachline-state-uf308ht4` bucket (us-east-2), with an S3 lockfile alongside it. It contains the webhook signing secret, so treat the state bucket as sensitive.

## Usage

From the repo root:

```sh
make tf-stripe ENV=dev            # apply
make tf-stripe-destroy ENV=dev    # remove (archives products/prices, deletes links and the webhook endpoint)
make disable-stripe ENV=dev       # persistently turn the component off, then destroy
```

Outputs: `payment_links` (plan to URL), `price_ids` (lookup key to price ID) and `webhook_endpoint_id`.

## Notes

- Stripe only returns a webhook endpoint's signing secret when the endpoint is created. If the endpoint is replaced, Terraform writes the new secret, and the webhook Lambda reloads it on its next failed signature check (at most once a minute), so no payment-api redeploy is needed.
- Stripe does not let you change an existing endpoint's API version. Changing `webhook_api_version` replaces the endpoint, which also rotates the secret as above. Upgrade stripe-go in payment-api at the same time.
- Do not create webhook endpoints for this URL by hand in the dashboard. Two endpoints both receive every event, so each purchase would issue two licenses.
- Products cannot be deleted in Stripe once they have prices, so destroying this stack archives them. A later apply creates fresh ones.
