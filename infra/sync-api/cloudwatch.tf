# SNS Topic for CloudWatch Alarms
resource "aws_sns_topic" "alarms" {
  name = "breachline-sync-alarms"

  tags = {
    Name = "breachline-sync-alarms"
  }
}

# SNS Topic Subscription
resource "aws_sns_topic_subscription" "alarms_email" {
  topic_arn = aws_sns_topic.alarms.arn
  protocol  = "email"
  endpoint  = var.alarm_email
}

# Lambda Error Rate Alarms
resource "aws_cloudwatch_metric_alarm" "lambda_errors" {
  for_each = local.lambda_functions

  alarm_name          = "breachline-sync-${each.key}-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "Errors"
  namespace           = "AWS/Lambda"
  period              = 300
  statistic           = "Sum"
  threshold           = 5
  alarm_description   = "Alert when ${each.key} Lambda has more than 5 errors in 5 minutes"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  dimensions = {
    FunctionName = aws_lambda_function.functions[each.key].function_name
  }
}

# Change processor error alarm removed - lambda no longer exists

# API Gateway 5xx Error Alarm
resource "aws_cloudwatch_metric_alarm" "api_gateway_5xx" {
  alarm_name          = "breachline-sync-api-5xx-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "5XXError"
  namespace           = "AWS/ApiGateway"
  period              = 300
  statistic           = "Sum"
  threshold           = 10
  alarm_description   = "Alert when API Gateway has more than 10 5xx errors in 5 minutes"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  dimensions = {
    ApiName = aws_api_gateway_rest_api.main.name
    Stage   = aws_api_gateway_stage.v1.stage_name
  }
}

# API Gateway 4xx Error Alarm (for monitoring)
resource "aws_cloudwatch_metric_alarm" "api_gateway_4xx" {
  alarm_name          = "breachline-sync-api-4xx-errors"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 2
  metric_name         = "4XXError"
  namespace           = "AWS/ApiGateway"
  period              = 300
  statistic           = "Sum"
  threshold           = 100
  alarm_description   = "Alert when API Gateway has more than 100 4xx errors in 5 minutes"
  alarm_actions       = [aws_sns_topic.alarms.arn]
}

# DynamoDB Throttled Requests Alarm
resource "aws_cloudwatch_metric_alarm" "dynamodb_throttles" {
  for_each = {
    pins              = aws_dynamodb_table.pins.name
    workspaces        = aws_dynamodb_table.workspaces.name
    workspace_members = aws_dynamodb_table.workspace_members.name
    annotations       = aws_dynamodb_table.annotations.name
    audit             = aws_dynamodb_table.audit.name
    workspace_files   = aws_dynamodb_table.workspace_files.name
    file_locations    = aws_dynamodb_table.workspace_file_locations.name
    subscriptions     = aws_dynamodb_table.user_subscriptions.name
  }

  alarm_name          = "breachline-sync-dynamodb-${each.key}-throttles"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "UserErrors"
  namespace           = "AWS/DynamoDB"
  period              = 300
  statistic           = "Sum"
  threshold           = 10
  alarm_description   = "Alert when ${each.key} table has more than 10 throttled requests"
  alarm_actions       = [aws_sns_topic.alarms.arn]

  dimensions = {
    TableName = each.value
  }
}

# Note: SQS Queue Age Alarm removed as part of direct DynamoDB operations refactor

# Lambda Concurrent Executions Alarm
resource "aws_cloudwatch_metric_alarm" "lambda_concurrent_executions" {
  alarm_name          = "breachline-sync-lambda-concurrent-executions"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = 1
  metric_name         = "ConcurrentExecutions"
  namespace           = "AWS/Lambda"
  period              = 60
  statistic           = "Maximum"
  threshold           = 900  # Alert at 90% of default limit (1000)
  alarm_description   = "Alert when Lambda concurrent executions are high"
  alarm_actions       = [aws_sns_topic.alarms.arn]
}

# Change processor duration alarm removed - lambda no longer exists

# Custom CloudWatch Dashboard
resource "aws_cloudwatch_dashboard" "main" {
  dashboard_name = "breachline-sync-api"

  dashboard_body = jsonencode({
    widgets = [
      {
        type = "metric"
        properties = {
          metrics = [
            ["AWS/ApiGateway", "Count", { stat = "Sum", label = "API Requests" }],
            [".", "4XXError", { stat = "Sum", label = "4xx Errors" }],
            [".", "5XXError", { stat = "Sum", label = "5xx Errors" }],
          ]
          period = 300
          stat   = "Sum"
          region = var.aws_region
          title  = "API Gateway Metrics"
        }
      },
      {
        type = "metric"
        properties = {
          metrics = [
            ["AWS/Lambda", "Invocations", { stat = "Sum", label = "Invocations" }],
            [".", "Errors", { stat = "Sum", label = "Errors" }],
            [".", "Throttles", { stat = "Sum", label = "Throttles" }],
          ]
          period = 300
          stat   = "Sum"
          region = var.aws_region
          title  = "Lambda Metrics"
        }
      },
      {
        type = "metric"
        properties = {
          metrics = [
            ["AWS/DynamoDB", "ConsumedReadCapacityUnits", { stat = "Sum" }],
            [".", "ConsumedWriteCapacityUnits", { stat = "Sum" }],
            [".", "UserErrors", { stat = "Sum" }],
          ]
          period = 300
          stat   = "Sum"
          region = var.aws_region
          title  = "DynamoDB Metrics"
        }
      },
      {
        type = "metric"
        properties = {
          metrics = [
            ["AWS/SQS", "NumberOfMessagesSent", { stat = "Sum" }],
            [".", "NumberOfMessagesReceived", { stat = "Sum" }],
            [".", "ApproximateNumberOfMessagesVisible", { stat = "Average" }],
          ]
          period = 300
          stat   = "Sum"
          region = var.aws_region
          title  = "SQS Metrics"
        }
      }
    ]
  })
}
