# Rate Limiting Terraform Configuration

# Rate limits DynamoDB table
resource "aws_dynamodb_table" "rate_limits" {
  name           = "${var.project}-rate-limits"
  billing_mode   = "PAY_PER_REQUEST"
  hash_key       = "license_key_hash"
  range_key      = "endpoint"
  
  attribute {
    name = "license_key_hash"
    type = "S"
  }
  
  attribute {
    name = "endpoint"
    type = "S"
  }

  # Enable TTL for automatic cleanup of expired entries
  ttl {
    attribute_name = "ttl"
    enabled        = true
  }

  tags = {
    Project     = var.project
    Environment = var.environment
    Purpose     = "rate-limiting"
  }
}

# CloudWatch alarm for high rate limit throttling
resource "aws_cloudwatch_metric_alarm" "rate_limit_throttling" {
  alarm_name          = "${var.project}-rate-limit-throttling"
  comparison_operator = "GreaterThanThreshold"
  evaluation_periods  = "2"
  metric_name         = "RateLimitExceeded"
  namespace           = "BreachLine/SyncAPI"
  period              = "300"  # 5 minutes
  statistic           = "Sum"
  threshold           = "50"   # Alert if more than 50 rate limit violations in 5 minutes
  alarm_description   = "This metric monitors rate limit violations in the sync API"
  treat_missing_data  = "notBreaching"

  tags = {
    Project     = var.project
    Environment = var.environment
    Purpose     = "monitoring"
  }
}

# Custom metric for rate limit violations (to be published by lambda functions)
resource "aws_cloudwatch_log_metric_filter" "rate_limit_violations" {
  name           = "${var.project}-rate-limit-violations"
  log_group_name = aws_cloudwatch_log_group.sync_api.name
  pattern        = "Rate limit exceeded"

  metric_transformation {
    name      = "RateLimitExceeded"
    namespace = "BreachLine/SyncAPI"
    value     = "1"
  }
}

# Environment variables for rate limiting
locals {
  rate_limit_env_vars = {
    RATE_LIMITS_TABLE = aws_dynamodb_table.rate_limits.name
  }
}
