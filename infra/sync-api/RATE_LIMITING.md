# Rate Limiting Implementation

## Overview
Implemented comprehensive license-based rate limiting for the sync API using DynamoDB and middleware pattern.

## Architecture

### Rate Limiting Components

1. **DynamoDB Rate Limits Table**
   - Partition key: `license_key_hash` (from JWT token)
   - Sort key: `endpoint` (categorized API endpoint)
   - TTL enabled for automatic cleanup
   - Atomic operations for thread-safe counting

2. **Rate Limiting Middleware**
   - Extracts license key hash from authorizer context
   - Checks limits before processing requests
   - Returns 429 responses with detailed headers
   - Configurable limits per license tier

3. **License Tier Configuration**
   - Basic tier: 10 workspaces/min, 50 files/min, 100 annotations/min, 5 auth/min
   - Premium tier: 100 workspaces/min, 500 files/min, 1000 annotations/min, 10 auth/min
   - 60-second sliding windows

## Implementation Details

### API Functions Created

#### `/src/api/rate_limits.go`
- `RateLimitEntry` struct for DynamoDB storage
- `RateLimitConfig` for tier-based limits
- `CheckRateLimit()` with atomic DynamoDB operations
- `GetEndpointCategory()` for endpoint categorization
- `GetRateLimitStatus()` for monitoring

#### `/src/api/rate_limit_middleware.go`
- `RateLimitMiddleware` struct
- `CheckRateLimit()` method for request validation
- `CreateRateLimitErrorResponse()` for 429 responses
- `RateLimitedHandler()` wrapper function
- `ApplyRateLimitingToHandler()` convenience function

### Lambda Functions Updated

Applied rate limiting to key endpoints:
- `workspace-create` - Workspace creation operations
- `auth-refresh` - Token refresh operations  
- `file-create` - File upload operations
- `annotation-create` - Annotation creation operations

### Terraform Infrastructure

#### `/rate_limits.tf`
- DynamoDB table with TTL configuration
- CloudWatch alarms for throttling
- Log metric filters for monitoring

#### `/lambda.tf`
- Added `RATE_LIMITS_TABLE` environment variables
- Updated IAM policies for DynamoDB access
- Added rate limits table permissions

## Rate Limiting Algorithm

Uses sliding window with atomic DynamoDB operations:

```go
// Atomic increment with conditional check
input := &dynamodb.UpdateItemInput{
    TableName: aws.String(rateLimitsTable),
    Key: map[string]types.AttributeValue{
        "license_key_hash": &types.AttributeValueMemberS{Value: licenseHash},
        "endpoint":         &types.AttributeValueMemberS{Value: category},
    },
    UpdateExpression: aws.String("SET request_count = if_not_exists(request_count, :zero) + :one"),
    ConditionExpression: aws.String("request_count < :limit OR window_start < :current"),
    // ... additional attributes
}
```

## Response Headers

Rate limited responses include:
- `X-RateLimit-Limit`: Maximum requests per window
- `X-RateLimit-Remaining`: Requests remaining in current window
- `X-RateLimit-Reset`: Unix timestamp when window resets
- `Retry-After`: Seconds until next request allowed

## Monitoring & Alerting

### CloudWatch Metrics
- `RateLimitExceeded` custom metric from log filtering
- Alert triggers if >50 violations in 5 minutes

### Logging
- Detailed debug logs for rate limit checks
- Warning logs for blocked requests
- License hash truncation for privacy

## License Key Extraction

Rate limiting uses the `license_key_hash` from the JWT authorizer context:

```go
licenseKeyHash, ok := request.RequestContext.Authorizer["license_key_hash"].(string)
```

This ensures rate limiting is tied to specific licenses rather than just users.

## Configuration

### Environment Variables
- `RATE_LIMITS_TABLE`: DynamoDB table name

### Rate Limit Tiers
```go
basic: {
    workspaces: 10/min,
    files: 50/min, 
    annotations: 100/min,
    auth: 5/min
}
premium: {
    workspaces: 100/min,
    files: 500/min,
    annotations: 1000/min, 
    auth: 10/min
}
```

## Benefits

- **Scalable**: DynamoDB handles high throughput automatically
- **Distributed**: Works across multiple lambda instances
- **License-based**: Tied to specific license keys for fair usage
- **Flexible**: Different limits per endpoint and license tier
- **Auto-cleanup**: TTL removes expired entries automatically
- **Atomic**: Prevents race conditions with conditional writes
- **Observable**: Comprehensive monitoring and alerting

## Usage

To apply rate limiting to new lambda functions:

```go
func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Apply rate limiting
    rateLimitedHandler := api.ApplyRateLimitingToHandler(actualHandler, logger)
    return rateLimitedHandler(ctx, request)
}

func actualHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    // Your existing handler logic
}
```

## Files Modified

### New Files
- `/src/api/rate_limits.go` - Core rate limiting functions
- `/src/api/rate_limit_middleware.go` - Middleware implementation
- `/rate_limits.tf` - Terraform infrastructure

### Updated Files
- `/src/lambda_functions/workspace-create/main.go` - Added rate limiting
- `/src/lambda_functions/auth-refresh/main.go` - Added rate limiting
- `/src/lambda_functions/file-create/main.go` - Added rate limiting
- `/src/lambda_functions/annotation-create/main.go` - Added rate limiting
- `/lambda.tf` - Updated environment variables and IAM policies

## Deployment

1. Apply Terraform changes to create rate limits table
2. Deploy updated lambda functions with rate limiting
3. Monitor CloudWatch metrics for throttling
4. Adjust rate limits as needed based on usage patterns

The rate limiting system is now ready to protect your sync API from abuse while ensuring fair usage across different license tiers.
