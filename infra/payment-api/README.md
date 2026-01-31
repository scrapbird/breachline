# BreachLine Payment API

Unified infrastructure component for BreachLine's payment processing system. This component contains two Go-based AWS Lambda functions that handle license generation and Stripe webhook processing.

## Architecture

This infrastructure manages three Lambda functions:

### 1. License Generator
- **Purpose**: Generates signed JWT licenses for BreachLine customers
- **Runtime**: Go (provided.al2023)
- **Trigger**: SNS messages from license generation topic
- **Exposure**: Not exposed to internet - triggered only via SNS
- **Source**: `src/license-generator/`

### 2. Stripe Webhook Handler
- **Purpose**: Processes Stripe webhook events and publishes license requests
- **Runtime**: Go (provided.al2023)
- **Trigger**: API Gateway endpoint receiving Stripe webhooks
- **Exposure**: HTTPS endpoint exposed to internet (Stripe webhooks only)
- **Source**: `src/stripe-webhook/`

### 3. License Sender
- **Purpose**: Sends generated licenses to customers via email
- **Runtime**: Go (provided.al2023)
- **Trigger**: SNS messages from license delivery topic
- **Email Service**: Amazon SES
- **Sender Address**: `noreply@breachline.app`
- **Source**: `src/license-sender/`

### 4. SNS Topics
- **License Generation Topic**: Decouples webhook processing from license generation
  - Name: `breachline-license-generation`
- **License Delivery Topic**: Decouples license generation from email delivery
  - Name: `breachline-license-delivery`
- **Benefits**: Asynchronous processing, retry logic, scalability, fault tolerance

### 5. CloudFront Distribution
- **Purpose**: DDoS protection, cost optimization, and API Gateway access control
- **Price Class**: PriceClass_100 (North America & Europe only)
- **CloudFront Function**: Validates `stripe-signature` header at edge before forwarding
  - Returns 403 Forbidden if header is missing
  - Executes in < 1ms at CloudFront edge locations
  - Prevents requests without signature from reaching API Gateway/Lambda
- **Custom Secret Header**: CloudFront automatically adds a secret header to all origin requests
  - Unique 32-character random secret generated during deployment
  - Added as `x-cloudfront-secret` header to requests sent to API Gateway
  - Lambda validates this header before processing any request
  - **Blocks all direct API Gateway access** - requests must come through CloudFront
- **Caching Strategy**: 
  - ✅ Caches error responses (400, 403, 404, 429) for 5 minutes
  - ❌ Never caches successful webhook processing (200 OK)
- **Security Features**:
  - HTTPS only with TLSv1.2+
  - Security headers (HSTS, XSS protection, frame options)
  - Header validation at edge (before API Gateway)
  - Origin access control via secret header validation
- **Benefits**:
  - **DDoS Protection**: Invalid requests rejected at edge (200+ locations)
  - **Cost Savings**: Requests without signature never reach API Gateway or Lambda
  - **Global Distribution**: Malicious traffic absorbed at CloudFront edge
  - **Automatic Scaling**: CloudFront handles traffic spikes
  - **Access Control**: Direct API Gateway access completely blocked

## How It Works

### Purchase Flow
1. Customer completes purchase on Stripe Checkout
2. Stripe sends `checkout.session.completed` webhook to **CloudFront Distribution**
3. CloudFront adds custom secret header and forwards request to **API Gateway**
4. **Stripe Webhook Lambda** validates CloudFront secret header (blocks direct API Gateway access)
5. **Stripe Webhook Lambda** validates Stripe signature and extracts customer email
6. Webhook determines license duration based on subscription type:
   - **Monthly**: Calculates days until same date next month
   - **Yearly**: 365 days
7. Webhook publishes license generation request to **License Generation SNS Topic** (async)
8. SNS triggers **License Generator Lambda**
9. License Generator creates signed JWT license
10. License Generator publishes license data to **License Delivery SNS Topic** (async)
11. SNS triggers **License Sender Lambda**
12. License Sender sends email with license attachment via **Amazon SES**
13. Customer receives license file at their email address

**Benefits of SNS Architecture**:
- **Fully decoupled services** - each step is independent and asynchronous
- **Automatic retries** if any step fails
- **Dead letter queue support** for failed requests
- **Better scalability** - each Lambda can scale independently
- **Fault tolerance** - failures in one component don't affect others

## Prerequisites

1. **AWS Account** with appropriate permissions
2. **AWS CLI** configured with credentials
3. **Terraform** >= 1.0
4. **Go** >= 1.21 (for building Lambda functions)
5. **Stripe Account** with API keys
6. **S3 State Bucket**: `scrappy-tfstate` must exist in `ap-southeast-2`

## Setup Instructions

### 1. Configure Stripe

Before deploying, set up your Stripe account:

1. Go to [Stripe Dashboard > Products](https://dashboard.stripe.com/products)
2. Create product: "BreachLine Premium"
3. Create prices (monthly/yearly subscriptions)
4. Get your API keys from [API Keys](https://dashboard.stripe.com/apikeys)

### 2. Configure Terraform Variables

Copy the example file and configure your settings:

```bash
cd infra/payment-api
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
aws_region = "ap-southeast-2"
```

### 3. Build Lambda Functions

Build Lambda functions:

```bash
./build.sh
```

This compiles Go binaries and creates deployment packages.

### 4. Deploy Infrastructure

```bash
# Initialize Terraform (first time only)
terraform init

# Review planned changes
terraform plan

# Deploy infrastructure
terraform apply
```

### 5. Configure Secrets Manager

After deployment, populate the required secrets:

**License Signing Key:**
```bash
# Read the private key from scripts directory
PRIVATE_KEY=$(cat ../../scripts/license_private.pem)

# Update the secret
aws secretsmanager put-secret-value \
    --secret-id breachline-license-signing-key \
    --secret-string "$PRIVATE_KEY" \
    --region ap-southeast-2
```

**Stripe API Key:**
```bash
# Update with your Stripe API key (starts with sk_test_ or sk_live_)
aws secretsmanager put-secret-value \
    --secret-id breachline-stripe-api-key \
    --secret-string "sk_test_YOUR_STRIPE_API_KEY_HERE" \
    --region ap-southeast-2
```

**Stripe Webhook Secret:**
```bash
# Update with your Stripe webhook signing secret (starts with whsec_)
# Get this from Stripe Dashboard > Webhooks after creating the endpoint
aws secretsmanager put-secret-value \
    --secret-id breachline-stripe-webhook-secret \
    --secret-string "whsec_YOUR_WEBHOOK_SECRET_HERE" \
    --region ap-southeast-2
```

### 6. Understanding Terraform Outputs

After deployment, Terraform provides two webhook URLs:

```bash
# CloudFront URL - USE THIS for Stripe webhooks
terraform output webhook_url
# Example: https://d869b47jxwzup.cloudfront.net/webhook

# API Gateway URL - FOR REFERENCE ONLY (direct access blocked)
terraform output api_gateway_url
# Example: https://abc123.execute-api.ap-southeast-2.amazonaws.com/prod/webhook
```

**Which URL to use:**
- ✅ **`webhook_url` (CloudFront)**: Use this in Stripe Dashboard and all testing
  - Protected by DDoS protection and caching
  - Includes CloudFront secret header automatically
  - Global edge locations for low latency
  
- ❌ **`api_gateway_url` (Direct API Gateway)**: DO NOT USE - returns 403 Forbidden
  - Direct access is intentionally blocked for security
  - Missing CloudFront secret header
  - Provided for reference only

### 7. Configure Stripe Webhook

After deployment, register the webhook in Stripe:

```bash
# Get the CloudFront webhook URL (this is what you use in Stripe)
terraform output webhook_url
```

Then:
1. Go to [Stripe Dashboard > Webhooks](https://dashboard.stripe.com/webhooks)
2. Click "Add endpoint"
3. Enter the **CloudFront webhook URL** from the `webhook_url` output
4. Select event: `checkout.session.completed`
5. Copy the signing secret (starts with `whsec_`)
6. Update the AWS Secrets Manager secret:
   ```bash
   aws secretsmanager put-secret-value \
       --secret-id breachline-stripe-webhook-secret \
       --secret-string "whsec_YOUR_WEBHOOK_SECRET_HERE" \
       --region ap-southeast-2
   ```

## Usage

### License Generator

**Note**: The License Generator is not exposed to the internet. It is triggered by SNS messages published to the `breachline-license-generation` topic.

For testing purposes, you can publish a message to SNS to trigger license generation:

```bash
# Publish message to SNS topic
aws sns publish \
    --topic-arn arn:aws:sns:ap-southeast-2:$(aws sts get-caller-identity --query Account --output text):breachline-license-generation \
    --message '{"email":"customer@example.com","days":365}' \
    --region ap-southeast-2
```

This will trigger the License Generator Lambda, which will:
1. Generate a signed JWT license valid for the specified number of days
2. Publish the license data to the `breachline-license-delivery` SNS topic
3. Trigger the License Sender Lambda to email the license to the customer

You can view the results in CloudWatch Logs:
```bash
# View license generator logs
aws logs tail /aws/lambda/breachline-license-generator --follow --region ap-southeast-2

# View license sender logs
aws logs tail /aws/lambda/breachline-license-sender --follow --region ap-southeast-2
```

### Stripe Webhook

The webhook is automatically invoked by Stripe. To test:

```bash
# Install Stripe CLI and forward to CloudFront URL (required)
stripe listen --forward-to $(terraform output -raw webhook_url)

# Trigger test event
stripe trigger checkout.session.completed
```

**Important**: Always use the CloudFront webhook URL (`webhook_url` output), not the direct API Gateway URL. Direct API Gateway access is blocked for security.

## Monitoring

### CloudWatch Logs

View logs for either function:

```bash
# License Generator logs
aws logs tail $(terraform output -raw license_generator_log_group) --follow

# Stripe Webhook logs
aws logs tail $(terraform output -raw stripe_webhook_log_group) --follow
```

### Stripe Dashboard

Monitor webhook delivery at [Stripe Dashboard > Webhooks](https://dashboard.stripe.com/webhooks):
- View successful deliveries
- Retry failed webhooks
- Inspect event details

## Development

### Modifying Lambda Functions

All three Lambda functions are in the `src/` directory:
- `src/license-generator/main.go` - License generation logic
- `src/stripe-webhook/main.go` - Webhook processing logic
- `src/license-sender/main.go` - Email delivery logic

After making changes:

```bash
# Rebuild and redeploy
./build.sh
terraform apply
```

Terraform automatically detects changes to Go source files and rebuilds.

### Adding Dependencies

Update the `go.mod` file in the respective function directory:

```bash
cd src/license-generator  # or src/stripe-webhook or src/license-sender
go get github.com/example/package
go mod tidy
```

Then rebuild and redeploy.

## Troubleshooting

### Webhook Signature Verification Failed

**Cause**: Webhook secret mismatch or not set

**Solution**:
1. Get correct secret from Stripe Dashboard > Webhooks
2. Update AWS Secrets Manager:
   ```bash
   aws secretsmanager put-secret-value \
       --secret-id breachline-stripe-webhook-secret \
       --secret-string "whsec_YOUR_WEBHOOK_SECRET_HERE" \
       --region ap-southeast-2
   ```
3. Lambda will automatically use the new secret on next cold start

### Secret Value Not Set

**Cause**: License signing key not populated in Secrets Manager

**Solution**: Use `aws secretsmanager put-secret-value` to populate the secret

### Build Failures

**Cause**: Go dependencies not downloaded or compilation error

**Solution**:
```bash
# Clean and rebuild
rm -f license-generator.zip stripe-webhook.zip
./build.sh
```

### Lambda Timeout

**Cause**: Function taking too long (current limit: 30 seconds)

**Solution**: Increase `timeout` in `main.tf` for the affected function

### Direct API Gateway Access Returns 403 Forbidden

**Cause**: This is intentional - direct API Gateway access is blocked for security

**Solution**: 
- All requests **must** go through the CloudFront distribution
- Use the CloudFront URL from `terraform output webhook_url`
- Direct API Gateway access is blocked by CloudFront secret header validation
- If you need to test directly (not recommended for production):
  1. Temporarily disable CloudFront secret validation in Lambda code
  2. Redeploy the Lambda function
  3. Re-enable after testing

## Infrastructure Components

### Created Resources

- **3 Lambda Functions**: 
  - License Generator (triggered by SNS, publishes to SNS)
  - Stripe Webhook (triggered by API Gateway, publishes to SNS)
  - License Sender (triggered by SNS, sends via SES)
- **3 IAM Roles**: 
  - License Generator role with Secrets Manager and SNS publish permissions
  - Stripe Webhook role with Secrets Manager and SNS publish permissions
  - License Sender role with SES send email permissions
- **2 SNS Topics**: 
  - `breachline-license-generation` - webhook to generator
  - `breachline-license-delivery` - generator to sender
- **2 SNS Subscriptions**: Connect topics to respective Lambda functions
- **1 CloudFront Distribution**: DDoS protection, cost optimization, and API Gateway access control
- **1 Random Secret**: 32-character secret for CloudFront-to-API Gateway authentication
- **3 Secrets Manager Secrets**: 
  - `breachline-license-signing-key` - Stores license signing private key
  - `breachline-stripe-api-key` - Stores Stripe API secret key
  - `breachline-stripe-webhook-secret` - Stores Stripe webhook signing secret
- **1 API Gateway**: HTTPS endpoint behind CloudFront (direct access blocked)
- **3 CloudWatch Log Groups**: 14-day retention for each function

### Cost Estimates

- **CloudFront**: First 1TB free, then ~$0.085/GB (North America)
- **Lambda**: ~$0.20 per 1M requests + compute time (3 functions)
- **API Gateway**: ~$3.50 per 1M requests
- **SNS**: ~$0.50 per 1M requests + $0.09 per 100k email notifications
- **SES**: $0.10 per 1,000 emails sent
- **Secrets Manager**: ~$1.20 per month (3 secrets @ $0.40 each)
- **CloudWatch Logs**: ~$0.50/GB ingested + storage
- **Expected**: < $3.50/month for typical volume (< 1000 purchases/month)
- **DDoS Protection**: Cached errors prevent expensive Lambda invocations during attacks

## Security

1. **CloudFront DDoS Protection**: AWS Shield Standard automatically enabled
2. **CloudFront Function Validation**: Rejects requests without `stripe-signature` at edge (< 1ms)
3. **CloudFront Secret Header**: Unique 32-character random secret prevents direct API Gateway access
   - CloudFront adds `x-cloudfront-secret` header to all origin requests
   - Lambda validates secret before processing any request
   - Direct API Gateway access returns 403 Forbidden
   - Secret is auto-generated during deployment and stored in Lambda environment
4. **Error Response Caching**: Malicious/spam requests cached at edge (5 min TTL)
5. **API Gateway Rate Limiting**: 20 requests/second steady state, 100 concurrent burst limit
6. **API Gateway Request Validation**: Requires both `stripe-signature` and `x-cloudfront-secret` headers
7. **License Signing Key**: Stored securely in AWS Secrets Manager
8. **Stripe API Key**: Stored securely in AWS Secrets Manager (retrieved at Lambda init)
9. **Stripe Webhook Secret**: Stored securely in AWS Secrets Manager (retrieved at Lambda init)
10. **Webhook Signatures**: Always validated in Lambda before processing
11. **License Generator Isolation**: Not exposed to internet, only triggered via SNS
12. **SNS Message Security**: Only authorized publishers can publish to topics
13. **SES Email Security**: Only authorized sender address (noreply@breachline.app)
14. **HTTPS Only**: CloudFront and API Gateway enforce HTTPS with TLSv1.2+
15. **Security Headers**: HSTS, XSS protection, frame options via CloudFront
16. **Least Privilege**: IAM roles have minimal required permissions
17. **Network Segmentation**: Only CloudFront endpoint is public-facing
18. **Fully Decoupled Architecture**: No direct Lambda-to-Lambda invocations

## SES Configuration

**Important**: Before the system can send emails, you must:

1. **Verify the sender email** in Amazon SES:
   ```bash
   aws ses verify-email-identity --email-address noreply@breachline.app --region ap-southeast-2
   ```

2. **Check verification status**:
   ```bash
   aws ses get-identity-verification-attributes --identities noreply@breachline.app --region ap-southeast-2
   ```

3. **Move out of SES sandbox** (for production):
   - Go to AWS SES Console
   - Request production access
   - This allows sending to any email address (sandbox only allows verified addresses)

4. **Test email sending**:
   ```bash
   aws ses send-email \
       --from noreply@breachline.app \
       --destination ToAddresses=your-email@example.com \
       --message Subject={Data="Test"},Body={Text={Data="Test email"}} \
       --region ap-southeast-2
   ```

## Updating

To update the infrastructure:

1. Modify Terraform files in `infra/payment-api/`
2. Modify Lambda code in `src/license-generator/` or `src/stripe-webhook/`
3. Run `terraform plan` to review changes
4. Run `terraform apply` to deploy

Terraform will automatically rebuild Lambda functions when source code changes.

## Cleanup

To destroy all infrastructure:

```bash
# Remove build artifacts
rm -f license-generator.zip stripe-webhook.zip

# Destroy infrastructure
terraform destroy
```

**Important**: 
- Remove the webhook endpoint from Stripe Dashboard
- Secrets Manager secret has a 7-day recovery window

## Related Components

- **Website**: `../website/` - Contains Stripe Checkout integration
- **Signing Keys**: `../../scripts/generate_keys.py` - Generates keypairs
- **Application**: `../../application/` - Uses generated licenses

## Support

For issues:
1. Check CloudWatch logs for detailed error information
2. Review Stripe Dashboard for webhook delivery status
3. Verify Secrets Manager secret is populated
4. Check Terraform state for resource status
