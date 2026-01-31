# SES Domain Identity
resource "aws_ses_domain_identity" "main" {
  domain = var.ses_verified_domain
}

# SES Domain DKIM
resource "aws_ses_domain_dkim" "main" {
  domain = aws_ses_domain_identity.main.domain
}

# SES Email Identity for sending PINs
resource "aws_ses_email_identity" "from_email" {
  email = var.ses_email_from
}

# SES Configuration Set for tracking
resource "aws_ses_configuration_set" "main" {
  name = "breachline-sync-emails"

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
