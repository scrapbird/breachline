# Account-wide resources shared by the dev and prod stacks. These are
# singletons per AWS account/region, so declaring them in the per-env stacks
# would leave two (or four) state files fighting over one object. The env
# stacks reference them implicitly (SES sends from the verified domain, API
# Gateway stages log through the account's CloudWatch role).

#================================================
# SES domain identity (Easy DKIM)
#================================================

# Verified via the DKIM CNAMEs in the ses_dkim_records output. Once verified,
# SES allows sending from any address on the domain, so no per-address email
# identities are needed.
resource "aws_sesv2_email_identity" "domain" {
  email_identity = var.ses_domain
}

#================================================
# API Gateway CloudWatch logging role
#================================================

# IAM role API Gateway assumes to write execution logs to CloudWatch
resource "aws_iam_role" "api_gateway_cloudwatch" {
  name = "breachline-api-gateway-cloudwatch"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "apigateway.amazonaws.com"
        }
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "api_gateway_cloudwatch" {
  role       = aws_iam_role.api_gateway_cloudwatch.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonAPIGatewayPushToCloudWatchLogs"
}

# Region-wide API Gateway setting. Must exist before any env stack enables
# stage logging, which is why `make deploy` applies this layer first.
resource "aws_api_gateway_account" "main" {
  cloudwatch_role_arn = aws_iam_role.api_gateway_cloudwatch.arn

  depends_on = [aws_iam_role_policy_attachment.api_gateway_cloudwatch]
}
