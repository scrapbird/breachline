#!/bin/bash
# Deployment script for BreachLine Payment API

set -e

echo "=== BreachLine Payment API Deployment ==="
echo ""

# Check if terraform.tfvars exists
if [ ! -f terraform.tfvars ]; then
    echo "Error: terraform.tfvars not found!"
    echo "Please copy terraform.tfvars.example to terraform.tfvars and configure your settings."
    exit 1
fi

# Build Lambda functions
echo "Building Lambda functions..."
./build.sh

echo ""
echo "Initializing Terraform..."
terraform init

# Validate configuration
echo "Validating Terraform configuration..."
terraform validate

# Plan deployment
echo ""
echo "Planning deployment..."
terraform plan -out=tfplan

# Ask for confirmation
echo ""
read -p "Do you want to apply this plan? (yes/no): " confirm

if [ "$confirm" != "yes" ]; then
    echo "Deployment cancelled."
    rm -f tfplan
    exit 0
fi

# Apply the plan
echo ""
echo "Applying Terraform configuration..."
terraform apply tfplan

# Clean up
rm -f tfplan

# Show outputs
echo ""
echo "=== Deployment Complete ==="
echo ""
terraform output

echo ""
echo "IMPORTANT NEXT STEPS:"
echo ""
echo "1. Populate the license signing key in Secrets Manager:"
echo "   aws secretsmanager put-secret-value \\"
echo "       --secret-id breachline-license-signing-key \\"
echo "       --secret-string \"\$(cat ../../scripts/license_private.pem)\" \\"
echo "       --region ap-southeast-2"
echo ""
echo "2. Configure the webhook URL in Stripe Dashboard:"
echo "   - Go to https://dashboard.stripe.com/webhooks"
echo "   - Click 'Add endpoint'"
echo "   - Use the stripe_webhook_url from the output above"
echo "   - Select 'checkout.session.completed' event"
echo "   - Copy the webhook signing secret and update terraform.tfvars"
echo "   - Run 'terraform apply' again to update the Lambda environment"
