# SES Domain Identity. The domain (breachline.app) is a single account-level
# identity, so only prod declares/verifies it (see local.manage_ses_domain);
# dev reuses the same verified domain to send from noreply-dev@breachline.app.
resource "aws_ses_domain_identity" "main" {
  count  = local.manage_ses_domain ? 1 : 0
  domain = var.ses_verified_domain
}

# SES Domain DKIM
resource "aws_ses_domain_dkim" "main" {
  count  = local.manage_ses_domain ? 1 : 0
  domain = var.ses_verified_domain
}

# SES Email Identity for sending PINs
resource "aws_ses_email_identity" "from_email" {
  email = var.ses_email_from
}

# SES Configuration Set for tracking
resource "aws_ses_configuration_set" "main" {
  name = "breachline-sync-emails${local.suffix}"

  delivery_options {
    tls_policy = "Require"
  }
}

# CloudWatch event destination for SES
resource "aws_ses_event_destination" "cloudwatch" {
  name                   = "cloudwatch-destination"
  configuration_set_name = aws_ses_configuration_set.main.name
  enabled                = true
  matching_types         = ["send", "reject", "bounce", "complaint", "delivery"]

  cloudwatch_destination {
    default_value  = "default"
    dimension_name = "ses:configuration-set"
    value_source   = "messageTag"
  }
}
