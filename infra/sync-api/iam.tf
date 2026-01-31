# IAM role for Lambda functions
resource "aws_iam_role" "lambda_execution" {
  name = "breachline-sync-lambda-execution"

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

  tags = {
    Name = "breachline-sync-lambda-execution"
  }
}

# Basic Lambda execution policy
resource "aws_iam_role_policy_attachment" "lambda_basic" {
  role       = aws_iam_role.lambda_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# X-Ray tracing policy
resource "aws_iam_role_policy_attachment" "lambda_xray" {
  role       = aws_iam_role.lambda_execution.name
  policy_arn = "arn:aws:iam::aws:policy/AWSXRayDaemonWriteAccess"
}

# DynamoDB access policy
resource "aws_iam_role_policy" "lambda_dynamodb" {
  name = "breachline-sync-lambda-dynamodb"
  role = aws_iam_role.lambda_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan",
          "dynamodb:BatchGetItem",
          "dynamodb:BatchWriteItem",
          "dynamodb:ConditionCheckItem"
        ]
        Resource = [
          aws_dynamodb_table.pins.arn,
          aws_dynamodb_table.workspaces.arn,
          "${aws_dynamodb_table.workspaces.arn}/index/*",
          aws_dynamodb_table.workspace_members.arn,
          "${aws_dynamodb_table.workspace_members.arn}/index/*",
          aws_dynamodb_table.annotations.arn,
          "${aws_dynamodb_table.annotations.arn}/index/*",
          aws_dynamodb_table.workspace_files.arn,
          "${aws_dynamodb_table.workspace_files.arn}/index/*",
          aws_dynamodb_table.audit.arn,
          "${aws_dynamodb_table.audit.arn}/index/*"
        ]
      }
    ]
  })
}

# SQS access policy removed - using direct DynamoDB operations instead

# SES access policy for sending emails
resource "aws_iam_role_policy" "lambda_ses" {
  name = "breachline-sync-lambda-ses"
  role = aws_iam_role.lambda_execution.id

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
        Condition = {
          StringEquals = {
            "ses:FromAddress" = var.ses_email_from
          }
        }
      }
    ]
  })
}

# Secrets Manager access policy
resource "aws_iam_role_policy" "lambda_secrets" {
  name = "breachline-sync-lambda-secrets"
  role = aws_iam_role.lambda_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "secretsmanager:GetSecretValue"
        ]
        Resource = [
          aws_secretsmanager_secret.jwt_private_key.arn,
          aws_secretsmanager_secret.jwt_public_key.arn,
          aws_secretsmanager_secret.license_public_key.arn
        ]
      }
    ]
  })
}

# IAM role for API Gateway to invoke Lambda authorizer
resource "aws_iam_role" "api_gateway_cloudwatch" {
  name = "breachline-sync-api-gateway-cloudwatch"

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

  tags = {
    Name = "breachline-sync-api-gateway-cloudwatch"
  }
}

resource "aws_iam_role_policy_attachment" "api_gateway_cloudwatch" {
  role       = aws_iam_role.api_gateway_cloudwatch.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonAPIGatewayPushToCloudWatchLogs"
}
