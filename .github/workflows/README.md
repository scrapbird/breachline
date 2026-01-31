# GitHub Actions Build and Release Workflow

This directory contains the GitHub Actions workflow for automatically building and releasing BreachLine.

## Workflow Overview

The workflow is triggered when you push a tag matching the semantic versioning format (e.g., `v1.0.0`, `v2.1.3`).

### Jobs

1. **build-linux**: Builds the application for Linux (amd64)
   - Runs on `ubuntu-latest`
   - Uses Go 1.25.1 and Node.js 20
   - Installs GTK3 and WebKit2GTK dependencies
   - Installs Wails CLI and frontend dependencies
   - Builds with `wails build -tags webkit2_41 -platform linux/amd64`
   - Packages binary as `breachline-linux-amd64.tar.gz`
   - Uploads artifact for the release job

2. **build-windows**: Builds the application for Windows (amd64)
   - Runs on `windows-latest`
   - Uses Go 1.25.1 and Node.js 20
   - Installs Wails CLI and frontend dependencies
   - Builds with `wails build -platform windows/amd64`
   - Packages binary as `breachline-windows-amd64.zip`
   - Uploads artifact for the release job

3. **build-macos**: Builds the application for macOS (Universal Binary)
   - Runs on `macos-latest`
   - Uses Go 1.25.1 and Node.js 20
   - Installs Wails CLI and frontend dependencies
   - Builds with `wails build -platform darwin/universal`
   - Creates a Universal Binary supporting both Intel and Apple Silicon
   - Packages binary as `breachline-macos-universal.tar.gz`
   - Uploads artifact for the release job

4. **create-release**: Creates GitHub release with all binaries
   - Runs on `ubuntu-latest`
   - Waits for Linux, Windows, and macOS builds to complete
   - Downloads all build artifacts
   - Creates a GitHub release with the tag name
   - Uploads Linux, Windows, and macOS binaries to the release

## Prerequisites

GitHub Actions is automatically available for GitHub repositories. The workflow uses:

- **`secrets.GITHUB_TOKEN`**: Automatically provided by GitHub Actions (no setup required)
- **Permissions**: The workflow has `contents: write` permission to create releases

## Creating a Release

To trigger a build and release:

```bash
# Create and push a tag
git tag v1.0.0
git push origin v1.0.0
```

The tag MUST match the pattern `v<major>.<minor>.<patch>` (e.g., `v1.2.3`).

### Tag Format

- ✅ Valid: `v1.0.0`, `v2.1.3`, `v10.20.30`
- ❌ Invalid: `1.0.0`, `v1.0`, `release-1.0.0`, `v1.0.0-beta`

## Build Artifacts

The workflow produces the following artifacts:

- `breachline-linux-amd64.tar.gz` - Linux binary (tar.gz archive)
- `breachline-windows-amd64.zip` - Windows executable (zip archive)
- `breachline-macos-universal.tar.gz` - macOS application bundle (Universal Binary for Intel & Apple Silicon)

All artifacts are automatically uploaded to the GitHub release.

## Monitoring Builds

You can monitor the workflow execution:

1. Go to the "Actions" tab in your GitHub repository
2. Click on the "Build and Release" workflow
3. View the status of each job (build-linux, build-windows, create-release)

## Troubleshooting

### Build fails on Windows
- Check that Go and Node.js versions are compatible
- Verify Wails dependencies are correctly installed
- Review the Windows job logs in the Actions tab

### Build fails on Linux
- Ensure GTK3 and WebKit2GTK system libraries are available
- Check npm dependencies install correctly
- Review the Linux job logs in the Actions tab

### Build fails on macOS
- Check that Go and Node.js versions are compatible
- Verify Wails CLI installs correctly
- Review the macOS job logs in the Actions tab

### Release creation fails
- Verify the workflow has `contents: write` permission
- Check that both build jobs completed successfully
- Review the create-release job logs in the Actions tab

## Differences from CircleCI

Key advantages of GitHub Actions over CircleCI:

- **No external setup required**: Everything runs natively on GitHub
- **Automatic token**: `GITHUB_TOKEN` is provided automatically
- **Better artifact handling**: Built-in artifact upload/download actions
- **Free for public repos**: No credit/billing concerns for open source
- **Native GitHub integration**: Direct release creation without CLI tools

## Notes

- Builds only run on tag pushes, not regular commits
- All three platform builds (Linux, Windows, and macOS) must succeed before creating the release
- The workflow ignores all branch pushes (only responds to tags matching the pattern)
- Artifacts are automatically cleaned up after 90 days (GitHub default)
- The macOS build creates a Universal Binary that runs natively on both Intel and Apple Silicon Macs
