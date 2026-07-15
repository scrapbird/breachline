# BreachLine

![Icon](./application/build/appicon.png)

# About

BreachLine is a flexible tool for visualizing and analyzing time series data such as audit logs and security events. It is built for speed and ease of use during cyber incident response investigations.

It supports reading time series data from CSV, XLSX and JSON files, supporting custom JPATH expressions to locate and ingest the list data.

# Features

- Loads large files (up to ~5-10 GB), each row being a timestamped event
- Sort and query cache, allowing for fast re-sorting and re-querying of the data
- External sort, using a temporary file to sort the data, allowing for sorting of files larger than available memory
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

# Deploying Infrastructure

All infrastructure (`infra/payment-api`, `infra/sync-api`, `infra/website`) is deployed from the repo-root `Makefile`. A single committed `config.yml` (non-secret) plus `secrets.vault.yml` (ansible-vault encrypted) is the source of truth; `make render` fans them out to a sourced `secrets.env` and per-component `<env>.tfvars`.

Two environments live side by side in the **same AWS account** — every resource name and Terraform state key is suffixed `-dev` / `-prod`. `dev` points at the Stripe **sandbox**, `prod` at the **live** Stripe API. Pick the target with `ENV=dev|prod` (default `dev`).

```sh
make init                 # bootstrap config.yml + secrets.vault.yml + .vault-password-file
                          # (on a new machine, restore .vault-password-file from 1Password first)
make edit-secrets         # set vault.environments.<env>.stripe.{api_key,webhook_secret}
make deploy ENV=dev       # render + build lambdas + terraform apply the dev stack
make deploy-prod          # shorthand for `make deploy ENV=prod`
make help                 # list every target
```

- **Non-secret** per-environment config (domains, SES addresses, lambda sizing, component on/off toggles) lives in `config.yml`.
- **Secrets** (Stripe keys, plus auto-generated license + JWT ECDSA keypairs) live in `secrets.vault.yml`; they are wired straight into AWS Secrets Manager by Terraform, so a deploy needs no manual `put-secret-value` step.
- Rendered `secrets.env` and `infra/<component>/<env>.tfvars` are gitignored; `config.yml` and the encrypted `secrets.vault.yml` are committed. The vault password file (`.vault-password-file`) is **not** committed — keep it in 1Password.
- Deploys use the standard AWS credential chain, so run `aws sso login` (or export creds) before `make deploy`.

> Note: the SES domain identity for `breachline.app` is a single account-level resource, so only the `prod` stack declares/verifies it; `dev` sends from the same verified domain (`noreply-dev@breachline.app`).

