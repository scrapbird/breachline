#!/bin/bash
# Build script for Payment API Lambda functions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"

echo "========================================"
echo "Building Payment API Lambda Functions"
echo "========================================"

# Create build directory
echo ""
echo "Creating build directory..."
mkdir -p "$BUILD_DIR"
echo "✓ Build directory created at $BUILD_DIR"

# Build License Generator
echo ""
echo "Building License Generator..."
cd "$SCRIPT_DIR/src/license-generator"
go mod download
GOOS=linux GOARCH=arm64 go build -o bootstrap main.go
zip "$BUILD_DIR/license-generator.zip" bootstrap
rm bootstrap
echo "✓ License Generator built successfully"

# Build Stripe Webhook
echo ""
echo "Building Stripe Webhook..."
cd "$SCRIPT_DIR/src/stripe-webhook"
go mod download
GOOS=linux GOARCH=arm64 go build -o bootstrap main.go
zip "$BUILD_DIR/stripe-webhook.zip" bootstrap
rm bootstrap
echo "✓ Stripe Webhook built successfully"

# Build License Sender
echo ""
echo "Building License Sender..."
cd "$SCRIPT_DIR/src/license-sender"
go mod download
GOOS=linux GOARCH=arm64 go build -o bootstrap main.go
zip "$BUILD_DIR/license-sender.zip" bootstrap
rm bootstrap
echo "✓ License Sender built successfully"

echo ""
echo "========================================"
echo "✓ All functions built successfully!"
echo "========================================"
echo "Deployment packages:"
echo "  - ${BUILD_DIR}/license-generator.zip"
echo "  - ${BUILD_DIR}/stripe-webhook.zip"
echo "  - ${BUILD_DIR}/license-sender.zip"
echo ""
echo "Next steps:"
echo "1. Run 'terraform init' to initialize Terraform"
echo "2. Run 'terraform plan' to preview changes"
echo "3. Run 'terraform apply' to deploy the Lambda functions"
