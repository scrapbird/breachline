# Cloud Migration Analysis

Analysis of moving BreachLine infra off AWS, and a comparison of AWS, DigitalOcean and Cloudflare as hosts. Written 2026-10-08. Prices are list prices at time of writing; re-check before committing.

## Why

The new AWS account has been blocked pending support for months, so nothing can be deployed. Cost is not the driver: at current (pre-launch) volume AWS is mostly free tier.

## Current AWS footprint

| Component | AWS services used | Notes |
|---|---|---|
| `infra/website` | S3, CloudFront (function, cache/headers policies, OAC), ACM, Route53 | Hugo static site, ~4.2MB built |
| `infra/payment-api` | 3 Go Lambdas (`stripe-webhook`, `license-generator`, `license-sender`), API Gateway, 2 SNS topics chaining them, Secrets Manager (signing key, Stripe keys), SES (`SendRawEmail` with license attachment) | License signing is ECDSA, pure Go stdlib |
| `infra/sync-api` | 28 Go Lambdas + 1 authorizer Lambda, API Gateway REST (usage plan, request validator, throttling), 9 DynamoDB tables (~10 GSIs, 2 TTLs), Secrets Manager (license public key, JWT), SES (auth PINs), CloudWatch logs/alarms/dashboard, SNS alarm topic | `sqs.tf` is empty (SQS already removed). JWT via `golang-jwt` |
| Terraform state | S3 bucket `breachline-state-uf308ht4` + lockfile | |
| Deploy tooling | Root `Makefile`, `config.yml`, `secrets.vault.yml` | Provider-specific render + `terraform apply` |
| Desktop app | `application/app/sync/sync.go` holds the API base URL | No other AWS coupling in the app |

### DynamoDB tables

| Table | Keys | Secondary indexes | TTL |
|---|---|---|---|
| pins | email / pin_hash | | expires_at |
| workspaces | workspace_id | owner_email + created_at | |
| workspace_members | workspace_id / email | email + workspace_id | |
| annotations | workspace_id / annotation_id | file_hash, file_hash + jpath, color, created_by | |
| workspace_files | workspace_id / file_identifier | file_hash | |
| audit | audit_id / created_at | workspace_id + created_at | ttl |
| user_subscriptions | email | | |
| workspace_file_locations | instance_id / file_hash | workspace_id + file_hash | |
| rate_limits | (see `rate_limits.tf`) | | |

The data is plainly relational (workspaces, members, files, annotations). Every GSI maps directly to a SQL index. About 3.1k lines of Go in `infra/sync-api/src/api` touch the DynamoDB SDK, heaviest in `workspace.go`, `annotation.go`, `rate_limits.go`, `team.go` and `file.go`, with ~32 index/condition/transaction call sites.

## Option A: DigitalOcean

### Service mapping

| Need | AWS today | DigitalOcean |
|---|---|---|
| Static website + CDN | S3 + CloudFront + ACM | App Platform static site (3 free) or Spaces + built-in CDN |
| Compute | 31 Lambdas + API Gateway | One Go `net/http` binary on App Platform or a Droplet |
| Database | DynamoDB | Managed PostgreSQL (no DynamoDB equivalent; DO offers Postgres, MySQL, MongoDB, Valkey, Kafka) |
| Fan-out between payment steps | SNS | Direct function calls in one process |
| Auth at the edge | Lambda authorizer | Middleware |
| Rate limiting | API Gateway throttling + DynamoDB table | Middleware backed by Postgres or Valkey |
| TTL expiry | DynamoDB TTL | Periodic `DELETE ... WHERE expires_at < now()` (goroutine ticker or App Platform job) |
| Secrets | Secrets Manager | App Platform encrypted env vars, or ansible-vault injected at deploy |
| Email | SES | None. Droplets block outbound SMTP for new accounts. Use Postmark, Resend or SendGrid |
| Logs/alerts | CloudWatch | App Platform logs + DO Monitoring alerts, or a log forwarder |
| DNS | Route53 | DO DNS (free) or Cloudflare DNS |
| Terraform state | S3 | Spaces via Terraform `s3` backend with `skip_*` flags (verify lockfile support) |
| Serverless functions | Lambda | DO Functions exists (Go supported, 25k GiB-s free) but has no authorizers, no queue/topic triggers and weak Terraform support. Not recommended |
| Region | us-east-2 | `syd1` (matches the "closest to NZ" rule) |

### Work required

1. Rewrite the sync-api data layer from DynamoDB to Postgres (pgx + SQL migrations). Largest single task.
2. Collapse the 28 sync Lambdas + authorizer into one `http.ServeMux` with auth, rate-limit and request-validation middleware. Handlers already live in a shared `api` package, so this is mostly wiring.
3. Collapse payment-api into the same (or a second) service: Stripe webhook calls generate then send directly, no SNS.
4. Replace SES with an email provider in `license-sender` and the auth PIN flow.
5. Move the website to App Platform static site or Spaces + CDN; move DNS.
6. Rewrite Terraform with the `digitalocean` provider; update the `Makefile` and `config.yml` render logic; drop the `project=breachline` tag rule or map it to DO tags/projects.
7. Point the app at the new API. Keeping the `api.breachline.app` hostname means no app change.
8. Update `CLAUDE.md` infra rules that name AWS (state bucket, tags, "Go lambda" rule).

### Pros

- Simplest mental model: one Go binary + one Postgres. Matches the "Go + standard http library" backend rule.
- Sydney region.
- Easy local dev (run the binary + local Postgres).
- No lock-in: the same binary runs on any VPS or container host.

### Cons

- No scale-to-zero: a fixed monthly floor even with zero users.
- You own more operational detail (health checks, migrations, connection pooling).
- No native email.

## Option B: Cloudflare

### Service mapping

| Need | Cloudflare |
|---|---|
| Static website + CDN | Pages or Workers static assets (free) |
| Compute | Workers (JS/TS/WASM), or Containers (any image, incl. a Go binary) fronted by a Worker |
| Database | D1 (serverless SQLite, 10GB max per DB) or Postgres elsewhere via Hyperdrive |
| Fan-out | Queues, or direct calls |
| Auth / rate limiting | Worker middleware; Rate Limiting binding or Durable Objects |
| TTL expiry | Cron Triggers running a `DELETE` |
| Secrets | Worker secrets (free) |
| Email | Email Service send binding (public beta as of 2026; 3,000/mo included on Workers Paid, then $0.35/1k), or Postmark/Resend |
| Logs/alerts | Workers Logs + notifications |
| DNS | Cloudflare DNS (free) |
| Terraform state | R2 via Terraform `s3` backend |
| Region | Global edge (Auckland and Sydney PoPs); D1 primary placed near first writer, read replicas optional |

### The Go problem

Workers run JS/TS natively. Go only runs as WASM, and stdlib `net/http` servers and the AWS-style SDK patterns do not port cleanly (TinyGo has partial stdlib support). Realistically there are two paths:

- **B1: Rewrite the backend in TypeScript on Workers + D1.** Cleanest Cloudflare-native result and the cheapest to run. Stripe's Node SDK and WebCrypto (ECDSA P-256 signing, JWT) both work in Workers. Cost: a full rewrite of ~31 functions plus license generation, and it breaks the "always Go" backend rule. The license signing must stay byte-compatible with what the desktop app verifies.
- **B2: Keep Go in Cloudflare Containers behind a thin Worker.** Containers are billed per 10ms active with some usage included in Workers Paid. But D1 is only reachable from Workers bindings (or the slower HTTP API), so a Go container would want an external Postgres (Neon, Supabase, or DO) reached through Hyperdrive or directly. This brings back most of Option A's moving parts while adding Cloudflare's, so it has the downsides of both.

### Pros

- Very cheap at low and medium volume; true scale-to-zero.
- No egress fees (R2, D1).
- Free DNS, CDN, TLS, DDoS protection, which you'd likely put in front of any host anyway.
- D1 (SQLite) fits the relational schema; the DynamoDB-to-SQL rewrite is the same as for Postgres.

### Cons

- Go is a second-class citizen; B1 means a TypeScript rewrite.
- D1 limits: 10GB per DB, single-writer, SQLite dialect.
- Email sending is still beta.
- More vendor lock-in than a plain Go binary.

## Option C: Stay on AWS

If the account gets unblocked, nothing needs to change. The existing design is reasonable and almost free at current volume. The risk is the one already experienced: account-level support delays block all progress, and there's no fallback.

## Cost comparison

Estimated monthly cost at pre-launch / low volume (under ~1M API requests, under 1GB data, under 3,000 emails), per environment unless noted.

| Item | AWS | DigitalOcean | Cloudflare (B1) |
|---|---|---|---|
| Base plan | 0 | 0 | 5 (Workers Paid, shared by dev + prod) |
| Compute | ~0 (Lambda free tier) | 5-12 (App Platform basic container) or 4-6 (Droplet) | included |
| API gateway | ~0-3.50 (REST API, $3.50/M after free tier) | included | included |
| Database | ~0 (DynamoDB on-demand, 25GB free) | 15 (smallest managed Postgres) or ~0 extra if self-hosted on the Droplet | included (5GB, 25B reads, 50M writes) |
| Static site + CDN | ~0-1 | 0 (App Platform static) or 5 (Spaces) | 0 |
| Secrets | ~2.40 (6 x $0.40) | 0 | 0 |
| DNS | 0.50 per zone | 0 | 0 |
| Monitoring | ~0.70 (7 alarms) | 0 | 0 |
| Email | ~0 (SES $0.10/1k) | 0-15 (third party) | included (3k/mo, beta) |
| **Per env** | **~3-8** | **~20-35** (App Platform + managed DB) / **~6-10** (one Droplet, self-hosted Postgres) | **~0 incremental** |
| **dev + prod** | **~6-16** | **~40-70** / **~12-20** (both envs on one Droplet) or ~25-35 (one DB cluster, two databases) | **~5** |

At growth (say 50M requests/mo, 20GB data):

- AWS: API Gateway REST is the big line (~$175 at $3.50/M); switching to HTTP API cuts it to ~$50. DynamoDB and Lambda stay modest.
- DigitalOcean: mostly flat. Scale the container/Droplet and DB tier, ~$50-100.
- Cloudflare: ~$5 base + ~$12 requests + CPU + D1 storage overage, roughly $20-40.

## Effort comparison

| Task | DigitalOcean | Cloudflare B1 | Cloudflare B2 |
|---|---|---|---|
| Data layer rewrite (DynamoDB to SQL) | Yes (Go, Postgres) | Yes (TS, D1/SQLite) | Yes (Go, external Postgres) |
| Handler rewrite | No, rewire into one mux | Yes, full TS port | No, rewire into one mux |
| Authorizer / rate limiting | Go middleware | TS middleware / bindings | Go middleware + Worker shim |
| License signing | Unchanged Go | Port to WebCrypto, verify byte-compat | Unchanged Go |
| Email | Third-party provider | Email Service binding (beta) or provider | Provider |
| Terraform | New `digitalocean` provider | `cloudflare` provider + `wrangler` | Both, plus DB host |
| Deploy tooling (`Makefile`, `config.yml`) | Rework | Rework | Rework |
| Relative effort | Medium | High | Medium-high |
| Fits "Go + stdlib http" rule | Yes | No | Yes |

## Recommendation

1. **Go with DigitalOcean**, as one Go `net/http` service plus Postgres in `syd1`. It keeps Go, is the least risky rewrite, removes the AWS dependency entirely, and the binary stays portable.
2. **Use Cloudflare for the free parts anyway:** DNS, website hosting (Pages), and optionally a proxy in front of the DO API for TLS/DDoS/caching. This costs nothing and doesn't affect the backend choice.
3. Keep costs low by running dev and prod on one Droplet with self-hosted Postgres at first (~$12-20/mo total, Ansible-provisioned per the infra rules), and graduate prod to managed Postgres when there are paying users.
4. Revisit Cloudflare B1 only if per-request cost at scale becomes a real concern, or if a TypeScript backend becomes acceptable.

Whichever host is chosen, write the DynamoDB-to-SQL rewrite against `database/sql`-style interfaces so the DB host (managed Postgres, self-hosted, Neon) can change later without touching handlers.

## Unresolved questions

- Any existing AWS data to migrate, or greenfield?
- Dev and prod: separate infra, or shared Droplet/cluster?
- App Platform (managed) or Droplet (cheaper, Ansible)?
- One service or two (sync + payment)?
- Email provider choice?
- Is a TypeScript backend acceptable (needed for Cloudflare B1)?

## Sources

- [Cloudflare Workers pricing](https://developers.cloudflare.com/workers/platform/pricing/)
- [Cloudflare D1 pricing](https://developers.cloudflare.com/d1/platform/pricing/) and [limits](https://developers.cloudflare.com/d1/platform/limits)
- [Cloudflare Containers pricing](https://developers.cloudflare.com/containers/pricing/)
- [Cloudflare Email Service](https://developers.cloudflare.com/email-routing/email-sending/)
- [DigitalOcean free tier / pricing summary 2026](https://agentdeals.dev/digitalocean-free-tier-2026)
