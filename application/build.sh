#!/bin/bash
# Build the BreachLine desktop app (Wails) with the sync-API endpoint baked in
# at build time from the repo-root config.yml (api_domain_name for ENV).
#
#   ENV=prod ./build.sh                         # prod endpoint (default)
#   ENV=dev  ./build.sh                          # dev endpoint
#   ENV=prod ./build.sh -platform windows/amd64  # extra args pass through to wails
#
# The endpoint is injected via -ldflags into breachline/app/sync.BaseURL, so the
# distributed binary talks to the correct sync API without a runtime config.
set -euo pipefail

APP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$(cd "$APP_DIR/.." && pwd)"
ENV="${ENV:-prod}"

# Prefer the infra venv's python (has pyyaml); fall back to system python3.
PY="$REPO_DIR/scripts/venv/bin/python3"
[ -x "$PY" ] || PY="python3"

ENDPOINT="$("$PY" "$REPO_DIR/scripts/api-endpoint.py" "$ENV")"
echo "[build] ENV=$ENV  sync endpoint=$ENDPOINT"

cd "$APP_DIR"
wails build -ldflags "-X breachline/app/sync.BaseURL=$ENDPOINT" "$@"
echo "[build] done"
