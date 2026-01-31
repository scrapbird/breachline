# auth-request-pin Lambda Function

This Lambda function handles PIN-based authentication requests for the BreachLine Sync API.

## Functionality

1. Receives a license file (base64-encoded JWT) from the client
2. Validates the license signature using ECDSA public key
3. Extracts user email and quota information from the license JWT claims
4. Generates a cryptographically secure 6-digit PIN
5. Stores the hashed PIN in DynamoDB with 12-hour TTL
6. Sends the PIN to the user's email via AWS SES
7. Returns success response with masked email

## Environment Variables

- `PINS_TABLE`: DynamoDB table name for storing PINs
- `LICENSE_PUBLIC_KEY`: ARN of the ECDSA public key in Secrets Manager
- `SES_FROM_EMAIL`: Email address to send PINs from
- `SES_CONFIGURATION_SET`: SES configuration set name
- `PIN_TTL_HOURS`: PIN expiration time in hours (default: 12)
- `AWS_REGION`: AWS region

## Request Format

```json
{
  "license": "<base64-encoded-license-data>"
}
```

## Response Format

**Success (200)**:
```json
{
  "message": "PIN sent to u***@example.com",
  "expires_at": "2025-10-16T19:01:24Z",
  "pin_id": "pin_1697241684123456789"
}
```

**Error (400)**:
```json
{
  "error": {
    "code": "invalid_license",
    "message": "License is invalid or could not extract email"
  }
}
```

## License Format

The license file is a base64-encoded JWT token signed with ECDSA containing the following claims:
- `id`: Unique license identifier (UUID)
- `email`: User's email address
- `workspace_limit`: Number of workspaces allowed (optional)
- `seat_count`: Number of collaboration seats (optional)
- `exp`: License expiration timestamp (Unix epoch)
- `nbf`: License not-before timestamp (Unix epoch)

## Building

```bash
GOOS=linux GOARCH=arm64 go build -o bootstrap main.go
zip auth-request-pin.zip bootstrap
```

## IAM Permissions Required

- `dynamodb:PutItem` on PINs table
- `dynamodb:GetItem` on PINs table
- `secretsmanager:GetSecretValue` for license public key
- `ses:SendEmail` for sending PINs
- `logs:CreateLogGroup`, `logs:CreateLogStream`, `logs:PutLogEvents` for CloudWatch
