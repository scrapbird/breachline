# BreachLine

![Icon](./application/build/appicon.png)

# About

BreachLine is a flexible tool for visualizing and analyzing time series data such as audit logs and security events. It is built for speed and ease of use during cyber incident response investigations.

It supports reading time series data from CSV, XLSX and JSON files, supporting custom JPATH expressions to locate and ingest the list data.

# Features

- Loads large files (up to ~5-10 GB), each row being a timestamped event
- Sort and query cache, allowing for fast re-sorting and re-querying of the data
- Displays the events in a fast, responsive, virtualized data grid (only rendering visible rows, etc.)
- Provides filtering, sorting, navigation (seek by time)
- Includes a persistent search bar at the top that uses a filter language similar to Splunk SPL to filter rows in real time
- Renders graphs / histograms showing counts of events in time buckets (e.g. 1 hour, 5 minutes, etc.)
- Cross-platform: builds & runs on Windows, macOS, Linux
- Supports workspaces with saved annotations
- Allows annotated data to be exported to a combined timeline file
- Flexible timezone handling, including default ingest timezone and separate display timezone
- Normalizes timestamps to a configurable time format

# Repository Structure

The repository is structured as follows:

- [application](./application): The main application code
- [doc](./doc): Documentation
- [infra](./infra): Infrastructure terraform templates and supporting code
- [scripts](./scripts): Various helper scripts for generating licenses, test files and automating simple tasks go here

# Setup

Step-by-step to stand up **every component** (account layer, payment-api, sync-api, website) in a **brand-new AWS account** from scratch. The day-to-day command reference lives in [Deploying Infrastructure](#deploying-infrastructure) below; this section covers the one-time bootstrap around it.

## 0. Install local prerequisites

| Tool | Used for |
|---|---|
| AWS CLI | credentials for the target account (SSO or keys) |
| Terraform ≥ 1.0 | applies each infra component |
| Go ≥ 1.25 | compiles the Lambda zips (`build.sh`) |
| Hugo | builds the static website (`hugo --minify`) |
| Ansible | provides `ansible-vault` (encrypts `secrets.vault.yml`) |
| Python 3 (+ `venv`) | render/init scripts - `make venv` installs the rest |
| 1Password (or any secret store) | holds `.vault-password-file` off-repo |

## 1. Configure AWS credentials

Everything uses the standard AWS credential chain. Log in to the **new account** before deploying:

```sh
aws sso login            # or export AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
aws sts get-caller-identity   # confirm you're pointed at the right account
```

## 2. Bootstrap the Terraform state bucket (one-time, manual)

All components store state in the S3 bucket `breachline-state-uf308ht4` in `us-east-2`. It is **not** managed by Terraform (chicken-and-egg), so create it once by hand. State locking uses native S3 lockfiles (`use_lockfile = true`) - no DynamoDB table required.

```sh
aws s3api create-bucket --bucket breachline-state-uf308ht4 --region us-east-2 \
  --create-bucket-configuration LocationConstraint=us-east-2
aws s3api put-bucket-versioning --bucket breachline-state-uf308ht4 \
  --versioning-configuration Status=Enabled
```

## 3. DNS / Route53 (required for prod; skip for a dev-only bring-up)

Create a **public hosted zone for `breachline.app`** in Route53 and point your registrar's nameservers at it. This is required because the prod website reads the zone as a data source for its SSL/alias records, and SES DKIM records are added there. **Dev needs no DNS** - it serves on the CloudFront default domain (`enable_ssl = false`).

## 4. Initialize config + secrets

```sh
make init
```

Idempotent. It creates:
- **`.vault-password-file`** - random password for the vault. **Back this up to 1Password immediately**; it is the only key that decrypts `secrets.vault.yml` and is deliberately never committed.
- **`config.yml`** - non-secret config from the template (domains, emails, per-env component toggles).
- **`secrets.vault.yml`** - encrypted; auto-generates the license-signing and JWT **ECDSA P-256 keypairs** for dev + prod. Only the Stripe keys are filled in by hand (next steps).

## 5. Review non-secret config

```sh
make edit-config
```

Check domains, SES sender/from addresses, `alarm_email`, and the `components` on/off toggles per environment. For a first bring-up, consider deploying `dev` only (leave `prod` components off until DNS/SSL are ready).

## 6. Stripe - phase 1 (API key)

Create a Stripe account. Use the **sandbox** for `dev` (`sk_test_…`) and the **live** API for `prod` (`sk_live_…`). Grab the secret API key, then:

```sh
make edit-secrets    # set vault.environments.<env>.stripe.api_key
```

That is the only Stripe value set by hand. Products, prices, payment links and the webhook endpoint (with its signing secret) are created by [`infra/stripe`](infra/stripe/README.md) during the deploy.

## 7. Deploy dev

```sh
make deploy ENV=dev
```

Builds the Go Lambda zips + Hugo site, then applies **account → payment-api → stripe → sync-api → website** in that order:
- account: the shared SES domain identity and API Gateway logging role
- payment-api: the Stripe webhook Lambda, license signing and delivery
- stripe: the catalogue (premium, workspace sync packs, shared seats), premium payment links, and the webhook endpoint pointed at payment-api, whose signing secret it stores for the Lambda
- sync-api: validates licenses and signs JWTs
- website: built with the environment's payment links embedded, then uploaded to its bucket

## 8. Test the payment flow

```sh
stripe login                                 # once; pick the sandbox account
stripe trigger checkout.session.completed    # webhook -> license generator -> license email
```

`stripe trigger` uses `stripe@example.com`, which bounces once SES is out of the sandbox. To test delivery without hurting the SES sending reputation, publish straight to the license-generation topic with the SES mailbox simulator:

```sh
aws sns publish --region us-east-2 --profile breachline \
  --topic-arn arn:aws:sns:us-east-2:<account-id>:breachline-license-generation-dev \
  --subject "License Generation Request" \
  --message '{"email":"success@simulator.amazonses.com","days":30}'
```

## 9. SES (email delivery)

The account layer (`infra/account`, applied by every `make deploy` or alone with `make tf-account`) creates the `breachline.app` SES domain identity with Easy DKIM, shared by dev and prod. After its first apply:
- Add the DKIM CNAME records from `terraform output ses_dkim_records` (run in `infra/account`) to DNS.
- No per-address identities are needed: once the domain is verified, any `@breachline.app` sender works.
- New SES accounts are **sandboxed** per region - request production access in the stack's region (us-east-2) to email arbitrary customers.

## 10. Deploy prod

Once DNS, SSL and SES are ready and prod secrets are set:

```sh
make edit-secrets     # prod Stripe live API key (sk_live_)
make deploy ENV=prod
```

## 11. Verify / tear down

```sh
make help                # list every target
make destroy ENV=dev     # tear an environment down (reverse order)
```

## Desktop application

The BreachLine desktop app is independent of the infra above - see [`application/README.md`](./application/README.md) for building the Wails desktop binary and the `breachline-cli` TUI.

# Deploying Infrastructure

All infrastructure (`infra/account`, `infra/payment-api`, `infra/stripe`, `infra/sync-api`, `infra/website`) is deployed from the repo-root `Makefile`. A single committed `config.yml` (non-secret) plus `secrets.vault.yml` (ansible-vault encrypted) is the source of truth; `make render` fans them out to a sourced `secrets.env` and per-component `<env>.tfvars`.

Two environments live side by side in the **same AWS account** - every resource name and Terraform state key is suffixed `-dev` / `-prod`. `dev` points at the Stripe **sandbox**, `prod` at the **live** Stripe API. Pick the target with `ENV=dev|prod` (default `dev`).

```sh
make init                 # bootstrap config.yml + secrets.vault.yml + .vault-password-file
                          # (on a new machine, restore .vault-password-file from 1Password first)
make edit-secrets         # set vault.environments.<env>.stripe.api_key
make deploy ENV=dev       # render + build lambdas + terraform apply the dev stack
make deploy-prod          # shorthand for `make deploy ENV=prod`
make build-app-dev        # desktop app against the dev sync API (raw API Gateway URL, no DNS name)
make build-app-prod       # desktop app against the prod sync API (api.breachline.app)
make help                 # list every target
```

- **Non-secret** per-environment config (domains, SES addresses, lambda sizing, component on/off toggles) lives in `config.yml`.
- **Secrets** (the Stripe API key, plus auto-generated license + JWT ECDSA keypairs) live in `secrets.vault.yml`; they are wired straight into AWS Secrets Manager by Terraform, so a deploy needs no manual `put-secret-value` step.
- Rendered `secrets.env` and `infra/<component>/<env>.tfvars` are gitignored; `config.yml` and the encrypted `secrets.vault.yml` are committed. The vault password file (`.vault-password-file`) is **not** committed - keep it in 1Password.
- Every `aws`/`terraform` call in the Makefile uses the `breachline` AWS profile (override with `AWS_PROFILE_NAME=<profile>`), so run `aws sso login --profile breachline` before `make deploy`.

> Note: account-wide resources (the `breachline.app` SES domain identity and the API Gateway CloudWatch role) live in `infra/account`, applied once and shared by both environments.
