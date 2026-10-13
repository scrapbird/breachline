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
