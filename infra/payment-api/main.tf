terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.0"
    }
  }

  backend "s3" {
    bucket       = "scrappy-tfstate"
    key          = "breachline-payment-api/terraform.tfstate"
    region       = "ap-southeast-2"
    use_lockfile = true
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      project   = "breachline"
      component = "payment-api"
    }
  }
}

# Data source for current AWS account
data "aws_caller_identity" "current" {}

# Random secret for CloudFront to API Gateway authentication
resource "random_password" "cloudfront_secret" {
  length  = 32
  special = true
}

#================================================
# License Generator Lambda Function
#================================================

# Secrets Manager secret for the license signing key
resource "aws_secretsmanager_secret" "signing_key" {
  name        = "breachline-license-signing-key"
  description = "Private key for signing BreachLine licenses"

  recovery_window_in_days = 7
}

# Secrets Manager secret for Stripe API key
resource "aws_secretsmanager_secret" "stripe_api_key" {
  name        = "breachline-stripe-api-key"
  description = "Stripe API key for payment processing"

  recovery_window_in_days = 7
}

# Secrets Manager secret for Stripe webhook secret
resource "aws_secretsmanager_secret" "stripe_webhook_secret" {
  name        = "breachline-stripe-webhook-secret"
  description = "Stripe webhook signing secret for verifying webhook authenticity"

  recovery_window_in_days = 7
}

# IAM role for License Generator Lambda
resource "aws_iam_role" "license_generator_role" {
  name = "breachline-license-generator-lambda"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

# IAM policy for License Generator to access Secrets Manager
resource "aws_iam_role_policy" "license_generator_secrets" {
  name = "secrets-access"
  role = aws_iam_role.license_generator_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue"
        ]
        Resource = aws_secretsmanager_secret.signing_key.arn
      }
    ]
  })
}

# Attach basic Lambda execution policy to License Generator
resource "aws_iam_role_policy_attachment" "license_generator_basic" {
  role       = aws_iam_role.license_generator_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# IAM policy for License Generator to publish to SNS
resource "aws_iam_role_policy" "license_generator_sns_publish" {
  name = "sns-publish"
  role = aws_iam_role.license_generator_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sns:Publish"
        ]
        Resource = aws_sns_topic.license_delivery.arn
      }
    ]
  })
}

# Build License Generator Lambda
resource "null_resource" "build_license_generator" {
  triggers = {
    main_go = filemd5("${path.module}/src/license-generator/main.go")
    go_mod  = filemd5("${path.module}/src/license-generator/go.mod")
  }

  provisioner "local-exec" {
    command = "cd ${path.module}/src/license-generator && GOOS=linux GOARCH=arm64 go build -o bootstrap main.go && zip ../../build/license-generator.zip bootstrap && rm bootstrap"
  }
}

# License Generator Lambda function
resource "aws_lambda_function" "license_generator" {
  filename         = "${path.module}/build/license-generator.zip"
  function_name    = "breachline-license-generator"
  role             = aws_iam_role.license_generator_role.arn
  handler          = "bootstrap"
  source_code_hash = filebase64sha256("${path.module}/build/license-generator.zip")
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  timeout          = 30
  memory_size      = 256

  environment {
    variables = {
      LOG_LEVEL              = "INFO"
      LICENSE_DELIVERY_TOPIC = aws_sns_topic.license_delivery.arn
    }
  }

  depends_on = [null_resource.build_license_generator]
}

# CloudWatch Log Group for License Generator
resource "aws_cloudwatch_log_group" "license_generator_logs" {
  name              = "/aws/lambda/${aws_lambda_function.license_generator.function_name}"
  retention_in_days = 14
}

#================================================
# SNS Topics
#================================================

# SNS topic for license generation requests
resource "aws_sns_topic" "license_generation" {
  name = "breachline-license-generation"
}

# SNS topic for license delivery notifications
resource "aws_sns_topic" "license_delivery" {
  name = "breachline-license-delivery"
}

# Subscribe license generator Lambda to SNS topic
resource "aws_sns_topic_subscription" "license_generator_subscription" {
  topic_arn = aws_sns_topic.license_generation.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.license_generator.arn
}

# Allow SNS to invoke the license generator Lambda
resource "aws_lambda_permission" "license_generator_sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.license_generator.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.license_generation.arn
}

#================================================
# API Gateway CloudWatch Logging Setup
#================================================

# IAM role for API Gateway to write logs to CloudWatch
resource "aws_iam_role" "api_gateway_cloudwatch" {
  name = "breachline-api-gateway-cloudwatch-role"

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

# Attach the AmazonAPIGatewayPushToCloudWatchLogs managed policy
resource "aws_iam_role_policy_attachment" "api_gateway_cloudwatch" {
  role       = aws_iam_role.api_gateway_cloudwatch.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonAPIGatewayPushToCloudWatchLogs"
}

# Set the CloudWatch role ARN in API Gateway account settings
resource "aws_api_gateway_account" "main" {
  cloudwatch_role_arn = aws_iam_role.api_gateway_cloudwatch.arn
}

#================================================
# Stripe Webhook Lambda Function
#================================================

# IAM role for Stripe Webhook Lambda
resource "aws_iam_role" "stripe_webhook_role" {
  name = "breachline-stripe-webhook-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

# Attach basic Lambda execution policy to Stripe Webhook
resource "aws_iam_role_policy_attachment" "stripe_webhook_basic" {
  role       = aws_iam_role.stripe_webhook_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# IAM policy for Stripe Webhook to publish to SNS
resource "aws_iam_role_policy" "stripe_webhook_sns_publish" {
  name = "sns-publish"
  role = aws_iam_role.stripe_webhook_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "sns:Publish"
        ]
        Resource = aws_sns_topic.license_generation.arn
      }
    ]
  })
}

# IAM policy for Stripe Webhook to access Secrets Manager
resource "aws_iam_role_policy" "stripe_webhook_secrets" {
  name = "secrets-access"
  role = aws_iam_role.stripe_webhook_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue"
        ]
        Resource = [
          aws_secretsmanager_secret.stripe_api_key.arn,
          aws_secretsmanager_secret.stripe_webhook_secret.arn
        ]
      }
    ]
  })
}

# Build Stripe Webhook Lambda
resource "null_resource" "build_stripe_webhook" {
  triggers = {
    main_go = filemd5("${path.module}/src/stripe-webhook/main.go")
    go_mod  = filemd5("${path.module}/src/stripe-webhook/go.mod")
  }

  provisioner "local-exec" {
    command = "cd ${path.module}/src/stripe-webhook && GOOS=linux GOARCH=arm64 go build -o bootstrap main.go && zip ../../build/stripe-webhook.zip bootstrap && rm bootstrap"
  }
}

# Stripe Webhook Lambda function
resource "aws_lambda_function" "stripe_webhook" {
  filename         = "${path.module}/build/stripe-webhook.zip"
  function_name    = "breachline-stripe-webhook"
  role             = aws_iam_role.stripe_webhook_role.arn
  handler          = "bootstrap"
  source_code_hash = filebase64sha256("${path.module}/build/stripe-webhook.zip")
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  timeout          = 30
  memory_size      = 256

  environment {
    variables = {
      STRIPE_WEBHOOK_SECRET_ARN    = aws_secretsmanager_secret.stripe_webhook_secret.arn
      LICENSE_GENERATION_TOPIC     = aws_sns_topic.license_generation.arn
      STRIPE_API_KEY_SECRET_ARN    = aws_secretsmanager_secret.stripe_api_key.arn
      CLOUDFRONT_SECRET            = random_password.cloudfront_secret.result
    }
  }

  depends_on = [null_resource.build_stripe_webhook]
}

# CloudWatch Log Group for Stripe Webhook
resource "aws_cloudwatch_log_group" "stripe_webhook_logs" {
  name              = "/aws/lambda/${aws_lambda_function.stripe_webhook.function_name}"
  retention_in_days = 14
}

# API Gateway REST API for Stripe Webhook
resource "aws_api_gateway_rest_api" "stripe_webhook" {
  name        = "breachline-stripe-webhook"
  description = "API Gateway for Stripe webhook handler"
}

# Request validator to ensure stripe-signature header is present
resource "aws_api_gateway_request_validator" "stripe_webhook_validator" {
  name                        = "stripe-signature-validator"
  rest_api_id                 = aws_api_gateway_rest_api.stripe_webhook.id
  validate_request_parameters = true
  validate_request_body       = false
}

# API Gateway Resource
resource "aws_api_gateway_resource" "webhook" {
  rest_api_id = aws_api_gateway_rest_api.stripe_webhook.id
  parent_id   = aws_api_gateway_rest_api.stripe_webhook.root_resource_id
  path_part   = "webhook"
}

# API Gateway Method (POST)
resource "aws_api_gateway_method" "webhook_post" {
  rest_api_id   = aws_api_gateway_rest_api.stripe_webhook.id
  resource_id   = aws_api_gateway_resource.webhook.id
  http_method   = "POST"
  authorization = "NONE"

  # Require stripe-signature and CloudFront secret headers
  request_parameters = {
    "method.request.header.stripe-signature" = true
    "method.request.header.x-cloudfront-secret" = true
  }

  request_validator_id = aws_api_gateway_request_validator.stripe_webhook_validator.id
}

# API Gateway Integration
resource "aws_api_gateway_integration" "lambda_integration" {
  rest_api_id             = aws_api_gateway_rest_api.stripe_webhook.id
  resource_id             = aws_api_gateway_resource.webhook.id
  http_method             = aws_api_gateway_method.webhook_post.http_method
  integration_http_method = "POST"
  type                    = "AWS_PROXY"
  uri                     = aws_lambda_function.stripe_webhook.invoke_arn
}

# API Gateway Deployment
resource "aws_api_gateway_deployment" "stripe_webhook_deployment" {
  rest_api_id = aws_api_gateway_rest_api.stripe_webhook.id

  depends_on = [
    aws_api_gateway_integration.lambda_integration,
  ]

  lifecycle {
    create_before_destroy = true
  }
}

# API Gateway Stage
resource "aws_api_gateway_stage" "stripe_webhook_stage" {
  deployment_id = aws_api_gateway_deployment.stripe_webhook_deployment.id
  rest_api_id   = aws_api_gateway_rest_api.stripe_webhook.id
  stage_name    = "prod"
}

# API Gateway Method Settings for Rate Limiting
resource "aws_api_gateway_method_settings" "webhook_throttling" {
  rest_api_id = aws_api_gateway_rest_api.stripe_webhook.id
  stage_name  = aws_api_gateway_stage.stripe_webhook_stage.stage_name
  method_path = "*/*"  # Apply to all methods and resources

  settings {
    throttling_burst_limit = 100  # Allow short bursts up to 100 concurrent requests
    throttling_rate_limit  = 20.0 # Steady state: 20 requests per second
    logging_level          = "INFO"
    data_trace_enabled     = false
    metrics_enabled        = true
  }
}

# Lambda permission for API Gateway
resource "aws_lambda_permission" "api_gateway" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.stripe_webhook.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.stripe_webhook.execution_arn}/*/*"
}

#================================================
# CloudFront Distribution for DDoS Protection
#================================================

# CloudFront Function to validate stripe-signature header
resource "aws_cloudfront_function" "stripe_signature_validator" {
  name    = "breachline-stripe-signature-validator"
  runtime = "cloudfront-js-2.0"
  comment = "Validate stripe-signature header exists before forwarding to API Gateway"
  publish = true
  code    = <<-EOT
async function handler(event) {
    var request = event.request;
    var headers = request.headers;
    
    // Check if stripe-signature header exists (case-insensitive)
    var hasStripeSignature = false;
    
    if (headers['stripe-signature']) {
        hasStripeSignature = true;
    }
    
    // If stripe-signature header is missing, return 403 Forbidden
    if (!hasStripeSignature) {
        return {
            statusCode: 403,
            statusDescription: 'Forbidden',
            headers: {
                'content-type': { value: 'application/json' },
                'cache-control': { value: 'public, max-age=300' }
            },
            body: JSON.stringify({
                error: 'Missing required header'
            })
        };
    }
    
    // Header exists, forward request to origin
    return request;
}
EOT
}

# CloudFront cache policy - cache errors, not successful webhooks
resource "aws_cloudfront_cache_policy" "webhook_cache_policy" {
  name        = "breachline-webhook-cache-policy"
  comment     = "Cache error responses only, never cache successful webhook processing"
  default_ttl = 300
  max_ttl     = 3600
  min_ttl     = 0

  parameters_in_cache_key_and_forwarded_to_origin {
    cookies_config {
      cookie_behavior = "none"
    }

    headers_config {
      header_behavior = "whitelist"
      headers {
        items = ["stripe-signature"]
      }
    }

    query_strings_config {
      query_string_behavior = "none"
    }

    enable_accept_encoding_gzip   = true
    enable_accept_encoding_brotli = true
  }
}

# CloudFront origin request policy
resource "aws_cloudfront_origin_request_policy" "webhook_origin_policy" {
  name    = "breachline-webhook-origin-policy"
  comment = "Forward webhook validation headers to API Gateway"

  cookies_config {
    cookie_behavior = "none"
  }

  headers_config {
    header_behavior = "whitelist"
    headers {
      items = [
        "stripe-signature",
        "content-type",
        "user-agent"
      ]
    }
  }

  query_strings_config {
    query_string_behavior = "none"
  }
}

# CloudFront response headers policy for security
resource "aws_cloudfront_response_headers_policy" "webhook_response_policy" {
  name    = "breachline-webhook-response-policy"
  comment = "Security headers for webhook endpoint"

  security_headers_config {
    strict_transport_security {
      access_control_max_age_sec = 31536000
      include_subdomains         = true
      override                   = true
    }

    content_type_options {
      override = true
    }

    frame_options {
      frame_option = "DENY"
      override     = true
    }

    xss_protection {
      mode_block = true
      protection = true
      override   = true
    }

    referrer_policy {
      referrer_policy = "strict-origin-when-cross-origin"
      override        = true
    }
  }
}

# CloudFront distribution
resource "aws_cloudfront_distribution" "webhook_distribution" {
  enabled         = true
  is_ipv6_enabled = true
  comment         = "CloudFront distribution for BreachLine Stripe webhooks - DDoS protection"
  price_class     = "PriceClass_100" # Use only North America and Europe edge locations

  origin {
    domain_name = "${aws_api_gateway_rest_api.stripe_webhook.id}.execute-api.${var.aws_region}.amazonaws.com"
    origin_id   = "APIGatewayOrigin"
    origin_path = "/${aws_api_gateway_stage.stripe_webhook_stage.stage_name}"

    custom_header {
      name  = "x-cloudfront-secret"
      value = random_password.cloudfront_secret.result
    }

    custom_origin_config {
      http_port              = 80
      https_port             = 443
      origin_protocol_policy = "https-only"
      origin_ssl_protocols   = ["TLSv1.2"]
    }
  }

  default_cache_behavior {
    allowed_methods  = ["GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"]
    cached_methods   = ["GET", "HEAD", "OPTIONS"]
    target_origin_id = "APIGatewayOrigin"

    cache_policy_id            = aws_cloudfront_cache_policy.webhook_cache_policy.id
    origin_request_policy_id   = aws_cloudfront_origin_request_policy.webhook_origin_policy.id
    response_headers_policy_id = aws_cloudfront_response_headers_policy.webhook_response_policy.id

    viewer_protocol_policy = "https-only"
    compress               = true

    # Only cache error responses (4xx, 5xx), never cache 2xx success responses
    min_ttl     = 0
    default_ttl = 0
    max_ttl     = 0

    # Attach CloudFront Function to validate stripe-signature header
    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.stripe_signature_validator.arn
    }
  }

  # Cache error responses to reduce load from spam/malicious requests
  custom_error_response {
    error_code            = 400
    error_caching_min_ttl = 300 # Cache 400 errors for 5 minutes
  }

  custom_error_response {
    error_code            = 403
    error_caching_min_ttl = 300 # Cache 403 errors for 5 minutes
  }

  custom_error_response {
    error_code            = 404
    error_caching_min_ttl = 300 # Cache 404 errors for 5 minutes
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    cloudfront_default_certificate = true
    minimum_protocol_version       = "TLSv1.2_2021"
  }

  tags = {
    Name = "breachline-webhook-cdn"
  }
}

#================================================
# License Sender Lambda Function
#================================================

# IAM role for License Sender Lambda
resource "aws_iam_role" "license_sender_role" {
  name = "breachline-license-sender-lambda-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

# Attach basic Lambda execution policy to License Sender
resource "aws_iam_role_policy_attachment" "license_sender_basic" {
  role       = aws_iam_role.license_sender_role.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# IAM policy for License Sender to send emails via SES
resource "aws_iam_role_policy" "license_sender_ses" {
  name = "ses-send-email"
  role = aws_iam_role.license_sender_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "ses:SendEmail",
          "ses:SendRawEmail"
        ]
        Resource = "*"
      }
    ]
  })
}

# Build License Sender Lambda
resource "null_resource" "build_license_sender" {
  triggers = {
    main_go = filemd5("${path.module}/src/license-sender/main.go")
    go_mod  = filemd5("${path.module}/src/license-sender/go.mod")
  }

  provisioner "local-exec" {
    command = "cd ${path.module}/src/license-sender && GOOS=linux GOARCH=arm64 go build -o bootstrap main.go && zip ../../build/license-sender.zip bootstrap && rm bootstrap"
  }
}

# License Sender Lambda function
resource "aws_lambda_function" "license_sender" {
  filename         = "${path.module}/build/license-sender.zip"
  function_name    = "breachline-license-sender"
  role             = aws_iam_role.license_sender_role.arn
  handler          = "bootstrap"
  source_code_hash = filebase64sha256("${path.module}/build/license-sender.zip")
  runtime          = "provided.al2023"
  architectures    = ["arm64"]
  timeout          = 30
  memory_size      = 256

  environment {
    variables = {
      SES_SENDER_EMAIL = "noreply@breachline.app"
      LOG_LEVEL        = "INFO"
    }
  }

  depends_on = [null_resource.build_license_sender]
}

# CloudWatch Log Group for License Sender
resource "aws_cloudwatch_log_group" "license_sender_logs" {
  name              = "/aws/lambda/${aws_lambda_function.license_sender.function_name}"
  retention_in_days = 14
}

# Subscribe license sender Lambda to license delivery SNS topic
resource "aws_sns_topic_subscription" "license_sender_subscription" {
  topic_arn = aws_sns_topic.license_delivery.arn
  protocol  = "lambda"
  endpoint  = aws_lambda_function.license_sender.arn
}

# Allow SNS to invoke the license sender Lambda
resource "aws_lambda_permission" "license_sender_sns" {
  statement_id  = "AllowSNSInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.license_sender.function_name
  principal     = "sns.amazonaws.com"
  source_arn    = aws_sns_topic.license_delivery.arn
}
