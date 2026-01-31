#!/bin/bash
# Build script for Sync API Lambda functions
# Usage: ./build.sh [function-name]
#   If function-name is provided, only that function will be built
#   Otherwise, all functions will be built

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_DIR="$SCRIPT_DIR/build"

# List of all Lambda functions
FUNCTIONS=(
  "auth-request-pin"
  "auth-verify-pin"
  "auth-refresh"
  "auth-logout"
  "workspace-create"
  "workspace-list"
  "workspace-get"
  "workspace-update"
  "workspace-delete"
  "workspace-convert-to-shared"
  "workspace-list-members"
  "workspace-add-member"
  "workspace-update-member"
  "workspace-remove-member"
  "annotation-list"
  "annotation-get"
  "annotation-create"
  "annotation-update"
  "annotation-delete"
  "file-list"
  "file-get"
  "file-create"
  "file-update"
  "file-delete"
  "authorizer"
  "file-location-store"
  "file-location-get"
  "file-locations-list"
)

# Check if a specific function was requested
if [ -n "$1" ]; then
  # Validate that the requested function exists in the list
  if [[ ! " ${FUNCTIONS[@]} " =~ " $1 " ]]; then
    echo "Error: Unknown function '$1'"
    echo ""
    echo "Available functions:"
    for func in "${FUNCTIONS[@]}"; do
      echo "  - $func"
    done
    exit 1
  fi
  
  # Build only the requested function
  FUNCTIONS=("$1")
  echo "========================================"
  echo "Building Lambda Function: $1"
  echo "========================================"
else
  echo "========================================"
  echo "Building Sync API Lambda Functions"
  echo "========================================"
fi

# Create build directory
echo ""
echo "Creating build directory..."
mkdir -p "$BUILD_DIR"
echo "✓ Build directory created at $BUILD_DIR"

# Build each function with concurrency
echo ""
echo "Starting parallel build of ${#FUNCTIONS[@]} functions..."

# Function to build a single component
build_component() {
  local func="$1"
  local script_dir="$2"
  local build_dir="$3"
  
  echo "[$func] Starting build..."
  
  if [ ! -d "$script_dir/src/lambda_functions/$func" ]; then
    echo "[$func] ⚠ Warning: Directory src/lambda_functions/$func does not exist, skipping..."
    return 0
  fi
  
  cd "$script_dir/src/lambda_functions/$func"

  # Run go mod tidy
  echo "[$func] Tidying go mod..."
  if ! go mod tidy 2>&1 | sed "s/^/[$func] /"; then
    echo "[$func] ✗ Failed to tidy go mod"
    return 1
  fi
  
  # Download dependencies
  echo "[$func] Downloading dependencies..."
  if ! go mod download 2>&1 | sed "s/^/[$func] /"; then
    echo "[$func] ✗ Failed to download dependencies"
    return 1
  fi
  
  # Build for ARM64
  echo "[$func] Building for ARM64..."
  if ! GOOS=linux GOARCH=arm64 go build -o bootstrap main.go 2>&1 | sed "s/^/[$func] /"; then
    echo "[$func] ✗ Build failed - compilation error"
    # Remove any partial build output
    [ -f "bootstrap" ] && rm -f "bootstrap"
    return 1
  fi
  
  # Verify bootstrap was created
  if [ ! -f "bootstrap" ]; then
    echo "[$func] ✗ Build failed - no output binary was created"
    return 1
  fi
  
  # Create deployment package in build directory
  echo "[$func] Creating deployment package..."
  if ! zip -j "$build_dir/${func}.zip" bootstrap 2>&1 | sed "s/^/[$func] /"; then
    echo "[$func] ✗ Failed to create deployment package"
    [ -f "bootstrap" ] && rm -f "bootstrap"
    return 1
  fi
  
  # Verify zip was created
  if [ ! -f "$build_dir/${func}.zip" ]; then
    echo "[$func] ✗ Failed to create deployment package - no zip file was created"
    [ -f "bootstrap" ] && rm -f "bootstrap"
    return 1
  fi
  
  # Clean up
  [ -f "bootstrap" ] && rm -f "bootstrap"
  
  echo "[$func] ✓ Built successfully ($build_dir/${func}.zip)"
  return 0
}

# Export function and variables for subshells
export -f build_component
export SCRIPT_DIR
export BUILD_DIR

# Create build status tracking file in the build directory
BUILD_STATUS_FILE="$BUILD_DIR/build_status.txt"
> "$BUILD_STATUS_FILE"

# Function to run build and capture status
run_build() {
  local func="$1"
  
  # Run the build
  if build_component "$func" "$SCRIPT_DIR" "$BUILD_DIR"; then
    echo "$func:0" >> "$BUILD_STATUS_FILE"
  else
    echo "$func:1" >> "$BUILD_STATUS_FILE"
  fi
}

export -f run_build
export BUILD_STATUS_FILE

# Get number of CPU cores for parallel builds
NUM_CORES=$(nproc 2>/dev/null || echo 2)
echo "Starting parallel builds using $NUM_CORES cores..."

# Run builds in parallel
printf '%s\n' "${FUNCTIONS[@]}" | xargs -P "$NUM_CORES" -n 1 -I {} bash -c 'run_build "$@"' _ {}

# Initialize failed builds counter
failed_builds=0

# Show build summary
echo ""
echo "========================================"
echo "Build Summary"
echo "========================================"

for func in "${FUNCTIONS[@]}"; do
  if [ ! -d "$SCRIPT_DIR/src/lambda_functions/$func" ]; then
    echo "➖ $func: Skipped (no source directory)"
    continue
  fi
  
  # Get build status from the status file if it exists
  status=1
  if [ -f "$BUILD_STATUS_FILE" ]; then
    status=$(grep -m1 "^$func:" "$BUILD_STATUS_FILE" | cut -d: -f2 || echo "1")
  fi
  
  if [ "$status" = "0" ] && [ -f "$BUILD_DIR/${func}.zip" ]; then
    # Check if zip is valid
    if unzip -t "$BUILD_DIR/${func}.zip" >/dev/null 2>&1; then
      echo "✅ $func: Built successfully"
    else
      echo "❌ $func: Built but zip file is corrupted"
      failed_builds=$((failed_builds + 1))
    fi
  else
    echo "❌ $func: Failed to build"
    if [ "$status" = "1" ]; then
      failed_builds=$((failed_builds + 1))
    fi
  fi
done

if [ $failed_builds -gt 0 ]; then
  echo ""
  echo "========================================"
  echo "✗ $failed_builds function(s) failed to build!"
  echo "========================================"
  exit 1
else
  echo ""
  echo "========================================"
  echo "✓ All functions built successfully!"
  echo "========================================"
fi
echo ""
echo "Deployment packages created in $BUILD_DIR"
echo ""
echo "Next steps:"
echo "1. Copy terraform.tfvars.example to terraform.tfvars and fill in values"
echo "2. Run 'terraform init' to initialize Terraform"
echo "3. Run 'terraform plan' to preview changes"
echo "4. Run 'terraform apply' to deploy the infrastructure"
