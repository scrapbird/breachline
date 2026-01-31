# BreachLine Sync API - Terraform Infrastructure

This directory contains Terraform templates for deploying the BreachLine Sync API infrastructure on AWS.

## Architecture Overview

The infrastructure consists of:

- **API Gateway**: REST API with JWT authentication
- **Lambda Functions**: 24+ serverless functions for all API endpoints
- **DynamoDB Tables**: 5 tables for data persistence
- **SQS FIFO Queue**: Ordered change processing with DLQ
- **SES**: Email delivery for PIN authentication
- **Secrets Manager**: Secure storage for JWT secrets and license keys
- **CloudWatch**: Monitoring, logging, and alarms

## Prerequisites

1. **AWS CLI** configured with appropriate credentials
2. **Terraform** >= 1.0 installed
3. **Go** >= 1.21 for building Lambda functions
4. **S3 Bucket** for Terraform state: `scrappy-tfstate`
5. **DynamoDB Table** for state locking: `terraform-state-lock`
6. **Verified SES Domain** or email address

## Required Input Variables

Before deploying, you must provide the following variables. Copy `terraform.tfvars.example` to `terraform.tfvars` and fill in the values:

```bash
cp terraform.tfvars.example terraform.tfvars
```

### Critical Variables

- `ses_email_from`: Email address to send PINs from (must be verified in SES)
- `ses_verified_domain`: Verified domain in SES
- `license_public_key`: ECDSA public key for license validation (PEM format)
- `jwt_secret`: Secure random string for signing JWT tokens
- `alarm_email`: Email address for CloudWatch alarms

### Optional Variables

- `aws_region`: AWS region (default: `ap-southeast-2`)
- `environment`: Environment name (default: `prod`)
- `api_domain_name`: Custom domain for API (optional)
- `lambda_memory_size`: Lambda memory in MB (default: `512`)
- `lambda_timeout`: Lambda timeout in seconds (default: `30`)

## Building Lambda Functions

Before deploying, you must build all Lambda functions:

```bash
./build.sh
```

## Deployment Steps

### 1. Initialize Terraform

```bash
terraform init
```

This will:
- Download required providers
- Configure S3 backend for state storage
- Set up state locking with DynamoDB

### 2. Review the Plan

```bash
terraform plan
```

Review the resources that will be created. Expected resources: ~100+

### 3. Deploy Infrastructure

```bash
terraform apply
```

Type `yes` when prompted to confirm deployment.

### 4. Configure DNS (if using custom domain)

After deployment, configure your DNS with the SES DKIM tokens:

```bash
terraform output ses_dkim_tokens
```

Add the DKIM CNAME records to your DNS provider.

### 5. Verify Deployment

```bash
# Get the API Gateway URL
terraform output api_gateway_url

# Test the API
curl -X POST https://your-api-url/v1/auth/request-pin \
  -H "Content-Type: application/json" \
  -d '{"license": "..."}'
```

## Post-Deployment Configuration

### Subscribe to Alarms

Confirm the SNS subscription sent to your alarm email:

```bash
# Check your email for AWS SNS subscription confirmation
```

### Monitor the Dashboard

Access the CloudWatch dashboard:

```bash
# Get dashboard name
terraform output dashboard_name

# Open in AWS Console
# Navigate to CloudWatch > Dashboards > breachline-sync-api
```

## State Management

Terraform state is stored in:
- **Bucket**: `scrappy-tfstate`
- **Key**: `sync-api/terraform.tfstate`
- **Region**: `ap-southeast-2`
- **Locking**: Enabled via DynamoDB

## Updating Infrastructure

To update the infrastructure:

```bash
# Pull latest changes
git pull

# Review changes
terraform plan

# Apply updates
terraform apply
```

## Updating Lambda Functions

To update Lambda function code without recreating infrastructure:

```bash
# Rebuild the function
cd src/auth-request-pin
GOOS=linux GOARCH=arm64 go build -o bootstrap main.go
zip function.zip bootstrap

# Update the Lambda
aws lambda update-function-code \
  --function-name breachline-sync-auth-request-pin \
  --zip-file fileb://function.zip \
  --region ap-southeast-2
```

## Destroying Infrastructure

**WARNING**: This will delete all data!

```bash
terraform destroy
```

## Troubleshooting

### Lambda Functions Not Found

Ensure all Lambda functions are built before running `terraform apply`:

```bash
ls -la src/*/function.zip
```

### SES Email Not Verified

Verify your email or domain in SES:

```bash
aws ses verify-email-identity --email-address noreply@breachline.com
```

### State Lock Issues

If state is locked:

```bash
# Force unlock (use with caution)
terraform force-unlock <LOCK_ID>
```

### DynamoDB Throttling

If you see throttling errors, DynamoDB will auto-scale. Monitor in CloudWatch.

## Monitoring and Alarms

The following CloudWatch alarms are configured:

- **Lambda Errors**: Triggers when any Lambda has >5 errors in 5 minutes
- **API Gateway 5xx**: Triggers when >10 5xx errors in 5 minutes
- **DLQ Messages**: Triggers immediately when messages appear in DLQ
- **DynamoDB Throttles**: Triggers when >10 throttled requests
- **SQS Message Age**: Triggers when messages are stuck >10 minutes

## Cost Estimation

Expected monthly costs (low usage):

- **Lambda**: ~$5-20 (pay per invocation)
- **DynamoDB**: ~$5-15 (on-demand pricing)
- **API Gateway**: ~$3.50 per million requests
- **SQS**: ~$0.40 per million requests
- **CloudWatch**: ~$5-10 (logs and metrics)
- **SES**: $0.10 per 1000 emails

**Total**: ~$20-50/month for low usage

## Security Considerations

- All DynamoDB tables use encryption at rest
- All API calls require TLS 1.2+
- JWT secrets stored in AWS Secrets Manager
- IAM roles follow least-privilege principle
- Lambda functions run outside VPC for performance
- API Gateway has rate limiting enabled

## Support

For issues or questions:
1. Check CloudWatch Logs for Lambda errors
2. Review CloudWatch Alarms for system issues
3. Check DLQ for failed message processing
4. Review API Gateway access logs

## Additional Resources

- [ARCHITECTURE.md](./doc/ARCHITECTURE.md) - Detailed architecture documentation
- [API_SPEC.md](./doc/API_SPEC.md) - API endpoint specifications
- [AWS Lambda Documentation](https://docs.aws.amazon.com/lambda/)
- [Terraform AWS Provider](https://registry.terraform.io/providers/hashicorp/aws/latest/docs)
