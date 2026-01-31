# Stripe Integration Setup Guide

This guide walks you through setting up Stripe payments for BreachLine software licenses.

## Overview

The integration allows customers to purchase BreachLine Premium licenses through Stripe Payment Links with two subscription options:
- **Monthly**: $10 USD/month
- **Yearly**: $60 USD/year (50% savings)

Both options support auto-renewal that customers can manage.

## Architecture Flow

```
Website (Hugo) 
  → Stripe Payment Link 
    → Payment Success 
      → Stripe Webhook 
        → Lambda Function 
          → Order API (http://api.breachline.app/order)
            → License Generation & Email Delivery
```

## Why Payment Links?

Stripe Payment Links provide several advantages over client-side Checkout integration:

- **No frontend dependencies**: No need to load Stripe.js or manage API keys on the website
- **Built-in features**: Support for promotion codes, custom fields, and tax collection configured in Stripe Dashboard
- **Easier to manage**: Update checkout settings without code changes
- **No parameter limitations**: All checkout customization done in Stripe Dashboard, avoiding integration errors
- **Faster implementation**: Simple redirect to Stripe-hosted page
- **Better security**: No client-side API key exposure

## Setup Steps

### 1. Create Stripe Account & Products

#### 1.1 Sign up for Stripe

1. Go to [https://stripe.com](https://stripe.com)
2. Create an account or sign in
3. Complete business verification (required for live mode)

#### 1.2 Create Payment Links

1. Go to **Payment Links** in Stripe Dashboard
2. Click **"+ New"** or **"Create payment link"**

3. Create **Monthly Payment Link**:
   - **Product**: Select existing or create new "BreachLine Premium"
   - **Price**: $10.00 USD
   - **Billing period**: Monthly (Recurring)
   - **Description**: Premium license for BreachLine incident response tool
   - **Collect customer addresses**: Enable if desired
   - **Allow promotion codes**: Enable if you want to support discount codes
   - **Custom fields**: Add any additional fields you need
   - Click **"Create link"**
   - **Copy the Payment Link URL** (e.g., `https://buy.stripe.com/abc123...`)

4. Create **Yearly Payment Link**:
   - Click **"+ New"** again
   - **Product**: Select "BreachLine Premium"
   - **Price**: $60.00 USD
   - **Billing period**: Yearly (Recurring)
   - **Description**: Yearly License (50% savings)
   - Configure same options as monthly
   - Click **"Create link"**
   - **Copy the Payment Link URL** (e.g., `https://buy.stripe.com/xyz789...`)

### 2. Get Stripe Secret Key

1. Go to **Developers > API keys** in Stripe Dashboard
2. Reveal and copy your **Secret key** (starts with `sk_test_` for test mode)

⚠️ **Important**: Keep the secret key secure! Never commit it to version control.

**Note**: When using Payment Links, you don't need the publishable key on the frontend. The secret key is only used by the webhook Lambda function to verify webhook signatures.

### 3. Configure Website

#### 3.1 Update Payment Links Configuration

Edit `/infra/website/src/themes/breachline-theme/static/js/stripe-checkout.js`:

```javascript
// Replace with your actual Payment Link URLs from Step 1.2
const PAYMENT_LINKS = {
    monthly: 'https://buy.stripe.com/YOUR_MONTHLY_LINK',
    yearly: 'https://buy.stripe.com/YOUR_YEARLY_LINK'
};
```

**Note**: No Stripe.js library or API keys are needed on the frontend when using Payment Links. The script simply redirects to the Stripe-hosted checkout page.

#### 3.2 Build and Deploy Website

```bash
cd infra/website/src
hugo
cd ..

# Get bucket name
BUCKET_NAME=$(terraform output -raw s3_bucket_name)

# Upload files
aws s3 sync ./src/public/ s3://$BUCKET_NAME/ --delete

# Invalidate CloudFront cache
DISTRIBUTION_ID=$(terraform output -raw cloudfront_distribution_id)
aws cloudfront create-invalidation \
  --distribution-id $DISTRIBUTION_ID \
  --paths "/*"
```

### 4. Deploy Stripe Webhook Handler

#### 4.1 Configure Terraform Variables

```bash
cd infra/stripe-webhook
cp terraform.tfvars.example terraform.tfvars
```

Edit `terraform.tfvars`:

```hcl
aws_region = "ap-southeast-2"

# From Step 2
stripe_secret_key = "sk_test_YOUR_SECRET_KEY_HERE"

# Leave this empty for now, will fill after first deployment
stripe_webhook_secret = ""

# Your order processing API endpoint
order_api_endpoint = "http://api.breachline.app/order"
```

#### 4.2 Deploy Infrastructure

```bash
# Initialize Terraform
terraform init

# Review the plan
terraform plan

# Deploy
terraform apply

# Save the webhook URL
terraform output webhook_url
# Example output: https://abc123.execute-api.ap-southeast-2.amazonaws.com/prod/webhook
```

### 5. Configure Stripe Webhook

#### 5.1 Add Webhook Endpoint

1. Go to **Developers > Webhooks** in Stripe Dashboard
2. Click **"Add endpoint"**
3. Enter webhook URL from previous step
4. Under **"Select events to listen to"**, choose:
   - ✓ `checkout.session.completed`
5. Click **"Add endpoint"**

#### 5.2 Get Webhook Signing Secret

1. Click on your newly created webhook endpoint
2. Click **"Reveal"** next to **Signing secret**
3. Copy the secret (starts with `whsec_`)

#### 5.3 Update Lambda with Webhook Secret

Edit `infra/stripe-webhook/terraform.tfvars`:

```hcl
stripe_webhook_secret = "whsec_YOUR_WEBHOOK_SECRET_HERE"
```

Redeploy:

```bash
terraform apply
```

### 6. Testing

#### 6.1 Test with Stripe Test Cards

Use these test card numbers:
- **Success**: `4242 4242 4242 4242`
- **Decline**: `4000 0000 0000 0002`
- **3D Secure**: `4000 0025 0000 3155`

Any future expiry date and any CVC will work.

#### 6.2 Complete a Test Purchase

1. Go to your website's pricing page
2. Click **"Purchase Monthly"** or **"Purchase Yearly"**
3. Enter test card details
4. Use any email address
5. Complete the checkout

#### 6.3 Verify Webhook Delivery

1. Go to **Developers > Webhooks** in Stripe Dashboard
2. Click on your webhook endpoint
3. Check the **"Recent deliveries"** section
4. Should show successful delivery (200 status)

#### 6.4 Check Lambda Logs

```bash
aws logs tail /aws/lambda/breachline-stripe-webhook --follow
```

### 7. Go Live Checklist

When ready to accept real payments:

#### 7.1 Switch to Live Mode in Stripe

1. Toggle from **Test mode** to **Live mode** in Stripe Dashboard
2. Complete business verification if not already done
3. Get your live secret key (starts with `sk_live_`) from **Developers > API keys**

#### 7.2 Create Live Payment Links

Repeat Step 1.2 in Live mode to create production Payment Links with live pricing.

#### 7.3 Update Website Configuration

Update `stripe-checkout.js` with the live Payment Link URLs.

#### 7.4 Update Lambda Configuration

Update `terraform.tfvars` with live secret key:

```hcl
stripe_secret_key = "sk_live_YOUR_LIVE_SECRET_KEY"
```

Deploy:
```bash
terraform apply
```

#### 7.5 Create Live Webhook

Repeat Step 5 to create a webhook endpoint in Live mode and update the webhook secret.

#### 7.6 Final Testing

Do a small test purchase with a real card to verify everything works end-to-end.

## Security Best Practices

### 1. Protect API Keys

- ✅ Never commit API keys to version control
- ✅ Use `terraform.tfvars` (gitignored) for secrets
- ✅ Use different keys for test and live modes
- ✅ Rotate keys periodically

### 2. Webhook Security

- ✅ Always verify webhook signatures (implemented in Lambda)
- ✅ Use HTTPS endpoints only (enforced by API Gateway)
- ✅ Keep webhook secrets secure

### 3. PCI Compliance

- ✅ Never handle raw card data (Stripe handles this)
- ✅ Use Stripe Checkout (PCI compliant by default)
- ✅ Don't log sensitive customer information

## Monitoring & Maintenance

### Check Webhook Health

Regularly check Stripe Dashboard > Webhooks for:
- Failed webhook deliveries
- Unusual patterns
- Error rates

### Monitor Lambda Function

```bash
# View recent logs
aws logs tail /aws/lambda/breachline-stripe-webhook --since 24h

# Watch live
aws logs tail /aws/lambda/breachline-stripe-webhook --follow
```

### Handle Failed Webhooks

If a webhook fails:
1. Check Lambda CloudWatch logs for error details
2. Fix the issue (e.g., order API down)
3. In Stripe Dashboard, find the failed webhook delivery
4. Click **"Resend"** to retry

### Update Dependencies

Periodically update Python dependencies:

```bash
cd infra/stripe-webhook
# Update versions in requirements.txt
vim requirements.txt

# Apply changes
terraform apply
```

## Common Issues

### Issue: "Payment configuration error"

**Cause**: Payment Link URLs not configured in `stripe-checkout.js`

**Fix**: Update `PAYMENT_LINKS` with actual Stripe Payment Link URLs from your Stripe Dashboard

### Issue: "Invalid signature"

**Cause**: Webhook secret mismatch

**Fix**: 
1. Get webhook secret from Stripe Dashboard
2. Update `terraform.tfvars`
3. Run `terraform apply`

### Issue: Customer doesn't receive license

**Cause**: Order API not processing orders correctly

**Fix**: Check order API logs and verify it's running

### Issue: Webhook not being called

**Cause**: 
- Wrong webhook URL in Stripe
- Lambda permission issues

**Fix**:
1. Verify webhook URL in Stripe matches Terraform output
2. Check Lambda CloudWatch logs
3. Test with Stripe CLI: `stripe trigger checkout.session.completed`

## Customer Experience Flow

1. Customer visits website pricing page
2. Clicks "Purchase Monthly" or "Purchase Yearly"
3. Redirected to Stripe-hosted Payment Link (secure, PCI compliant)
4. Enters payment information on Stripe's checkout page
5. Completes purchase
6. Redirected to success page (configured in Payment Link settings)
7. Receives license key via email (from order processor via webhook)

## Support

### For Stripe Issues
- [Stripe Documentation](https://stripe.com/docs)
- [Stripe Support](https://support.stripe.com)

### For Integration Issues
- Check CloudWatch logs
- Review Stripe webhook delivery logs
- Contact: noreply@breachline.app

## Next Steps

After completing this setup:

1. ✅ Integrate with license generator (`/infra/license-generator`)
2. ✅ Set up email delivery for license keys
3. ✅ Create customer portal for subscription management
4. ✅ Add analytics tracking for conversions
5. ✅ Implement customer support ticketing

## References

- [Stripe Checkout Documentation](https://stripe.com/docs/payments/checkout)
- [Stripe Webhooks Guide](https://stripe.com/docs/webhooks)
- [Stripe Testing Guide](https://stripe.com/docs/testing)
