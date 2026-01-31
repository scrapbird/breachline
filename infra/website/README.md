# BreachLine Website Infrastructure

This Terraform stack manages the infrastructure for the BreachLine static website.

## Architecture

The infrastructure consists of:
- **S3 Bucket**: Stores the static website files
- **CloudFront Distribution**: CDN for fast content delivery globally
- **Origin Access Control (OAC)**: Secures S3 bucket access to CloudFront only
- **ACM Certificate** (optional): SSL/TLS certificate for custom domains
- **Route53 Records** (optional): DNS records for custom domain

## Website Source Code

The website source code is located in the `src/` directory and is built using [Hugo](https://gohugo.io/), a fast and modern static site generator.

### Installing Hugo

Install Hugo using Go:

```bash
go install github.com/gohugoio/hugo@latest
```

Make sure `~/go/bin` is in your PATH:

```bash
export PATH=$PATH:~/go/bin
```

Verify the installation:

```bash
hugo version
```

### Building the Website

To build the static website:

```bash
cd src
hugo
```

This generates the static files in `src/public/` directory.

### Local Development

To run a local development server with live reload:

```bash
cd src
hugo server -D
```

Then open your browser to `http://localhost:1313`

### Website Content

The Hugo site includes:
- **Home page**: Overview of BreachLine
- **Features page**: Detailed feature descriptions
- **Download page**: Links to application downloads
- **Pricing page**: Free vs Premium comparison with Stripe Payment Links integration

Content is located in `src/content/` and can be edited as Markdown files.

### Stripe Payment Integration

The website uses Stripe Payment Links for handling premium subscriptions. The integration is configured in:
- **JavaScript**: `src/themes/breachline-theme/static/js/stripe-checkout.js`
- **Configuration**: Payment Link URLs are set in the `PAYMENT_LINKS` constant

To configure:
1. Create Payment Links in your Stripe Dashboard
2. Update the `PAYMENT_LINKS` object in `stripe-checkout.js` with your Payment Link URLs
3. No Stripe.js library or API keys needed on the frontend

For detailed setup instructions, see `/doc/STRIPE_INTEGRATION_SETUP.md`.

## Prerequisites

1. **AWS Account**: You need an AWS account with appropriate permissions
2. **AWS CLI**: Configured with credentials
3. **Terraform**: Version 1.0 or higher
4. **Hugo**: For building the website (see above)
5. **S3 State Bucket**: The `scrappy-tfstate` bucket must exist in `ap-southeast-2`

### Creating the State Backend Resources

If you haven't already, create the required backend resources:

```bash
# Create the S3 bucket for state storage
aws s3 mb s3://scrappy-tfstate --region ap-southeast-2

# Enable versioning on the state bucket
aws s3api put-bucket-versioning \
  --bucket scrappy-tfstate \
  --versioning-configuration Status=Enabled \
  --region ap-southeast-2
```

## Usage

### Initial Setup

1. **Copy the example variables file**:
   ```bash
   cp terraform.tfvars.example terraform.tfvars
   ```

2. **Edit `terraform.tfvars`** with your desired configuration:
   - Leave `domain_name` empty to use the CloudFront domain
   - Set `domain_name` and `enable_ssl = true` for a custom domain

3. **Initialize Terraform**:
   ```bash
   terraform init
   ```

4. **Review the plan**:
   ```bash
   terraform plan
   ```

5. **Apply the configuration**:
   ```bash
   terraform apply
   ```

### Deploying Website Content

After the infrastructure is created, build and upload your website files:

```bash
# Build the Hugo site
cd src
hugo
cd ..

# Get the bucket name from Terraform output
BUCKET_NAME=$(terraform output -raw s3_bucket_name)

# Sync the built website files to S3
aws s3 sync ./src/public/ s3://$BUCKET_NAME/ --delete

# Invalidate CloudFront cache
DISTRIBUTION_ID=$(terraform output -raw cloudfront_distribution_id)
aws cloudfront create-invalidation \
  --distribution-id $DISTRIBUTION_ID \
  --paths "/*"
```

### Custom Domain Setup

To use a custom domain:

1. Ensure your domain is registered and you have access to Route53 or your DNS provider
2. If using Route53, ensure the hosted zone exists for your domain
3. Set the following in `terraform.tfvars`:
   ```hcl
   domain_name = "yourdomain.com"
   subdomain   = "www"  # or "" for apex domain
   enable_ssl  = true
   ```
4. Run `terraform apply`
5. If not using Route53, create DNS records manually pointing to the CloudFront distribution

## Outputs

After applying, Terraform will output:

- `website_url`: The URL where your website is accessible
- `s3_bucket_name`: Name of the S3 bucket
- `cloudfront_distribution_id`: CloudFront distribution ID for cache invalidation
- `cloudfront_domain_name`: CloudFront domain name
- Additional outputs for certificates and DNS if custom domain is configured

## Updating the Website

To update website content:

1. Edit the content files in `src/content/` or modify the theme
2. Rebuild the site:
   ```bash
   cd src
   hugo
   cd ..
   ```
3. Upload the new files to S3:
   ```bash
   aws s3 sync ./src/public/ s3://$(terraform output -raw s3_bucket_name)/ --delete
   ```
4. Create a CloudFront invalidation to clear the cache:
   ```bash
   aws cloudfront create-invalidation \
     --distribution-id $(terraform output -raw cloudfront_distribution_id) \
     --paths "/*"
   ```

## Cost Considerations

- S3 storage and data transfer costs
- CloudFront data transfer and request costs
- Route53 hosted zone costs (if using custom domain)
- ACM certificates are free

The default `PriceClass_100` uses only North America and Europe edge locations, which is more cost-effective than global distribution.

## Cleanup

To destroy all resources:

```bash
# First, empty the S3 bucket
aws s3 rm s3://$(terraform output -raw s3_bucket_name) --recursive

# Then destroy the infrastructure
terraform destroy
```

## Troubleshooting

### Certificate Validation Pending

If you're using a custom domain and SSL, certificate validation may take several minutes. Terraform will wait for validation to complete.

### CloudFront Cache Issues

If you've updated files but don't see changes:
1. Create a CloudFront invalidation (see above)
2. Alternatively, wait for TTL to expire (default: 1 hour)
3. Or use hard refresh in your browser (Ctrl+Shift+R or Cmd+Shift+R)

### Permission Issues

Ensure your AWS credentials have permissions for:
- S3 (bucket creation, policy management)
- CloudFront (distribution creation)
- ACM (certificate management)
- Route53 (DNS management, if using custom domain)
