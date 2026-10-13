# BreachLine Account Layer

This Terraform stack manages the AWS resources that exist once per account and region and are shared by the `dev` and `prod` environments. Declaring them in the per-environment stacks would leave several state files managing the same object, so destroying one environment could break the other.

## Resources

- **SES domain identity** (`aws_sesv2_email_identity`) for the sending domain, using Easy DKIM. Once verified, SES allows sending from any address on the domain (for example `info@breachline.app`), so the environment stacks need no per-address email identities.
- **API Gateway CloudWatch logging role** (`aws_iam_role` + `aws_api_gateway_account`). This is a region-wide API Gateway setting, required before any stage can enable execution logging, which both `payment-api` and `sync-api` do.

All resources are tagged `project = breachline` and `component = account`.

## Input variables

| Variable | Description | Default |
|---|---|---|
| `aws_region` | Region for the resources. Must match the environment stacks, because SES identities and the API Gateway account setting are regional. | `us-east-2` |
| `ses_domain` | Domain verified in SES. | `breachline.app` |

Values come from `config.yml` (`config.aws.region` and `config.account.ses_domain`). `make render` writes them to `account.tfvars`, which is gitignored.

## State

State is stored at `account/terraform.tfstate` in the `breachline-state-uf308ht4` bucket (us-east-2), with an S3 lockfile alongside it. The key is fixed in `backend.tf` because this layer is not per-environment.

## Usage

From the repo root:

```sh
make tf-account          # render config, then init + apply this layer
```

`make deploy ENV=<env>` applies this layer first, before the environment components, so it never needs to be applied by hand in the normal flow. It is safe to re-apply.

After the first apply, add the DKIM records to DNS so SES can verify the domain:

```sh
cd infra/account && terraform output ses_dkim_records
```

Each entry is a CNAME (`<token>._domainkey.breachline.app` pointing to `<token>.dkim.amazonses.com`). If the domain is proxied through Cloudflare, set these records to DNS-only. Verification usually completes within an hour of the records resolving. New accounts start in the SES sandbox, so production access must also be requested for this region.

## Destroying

`make destroy ENV=<env>` deliberately leaves this layer alone. To remove it, destroy both environments first, then run `make tf-account-destroy`, which prompts for confirmation instead of auto-approving.
