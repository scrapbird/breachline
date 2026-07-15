#!/usr/bin/env bash
# Flip a config.environments.<env>.components.<key> boolean in config.yml in
# place, preserving the rest of the file (comments included). Used by the
# `make enable-*`/`disable-*` toggle targets so the intended on/off state
# persists across `make deploy`.
#
#   scripts/set-flag.sh <key> <true|false> <env>
#
# The components blocks for dev and prod share indentation, so we scope the
# in-place edit to the line range of the requested environment's block.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
CONFIG="$REPO/config.yml"
KEY="${1:?usage: set-flag.sh <key> <true|false> <env>}"
VAL="${2:?usage: set-flag.sh <key> <true|false> <env>}"
ENVIRONMENT="${3:?usage: set-flag.sh <key> <true|false> <env>}"

case "$VAL" in true|false) ;; *) echo "[error] value must be true|false" >&2; exit 1 ;; esac

# Line where the requested env block starts (e.g. "    dev:").
ENV_LINE=$(grep -nE "^    ${ENVIRONMENT}:[[:space:]]*$" "$CONFIG" | head -1 | cut -d: -f1 || true)
if [ -z "$ENV_LINE" ]; then
  echo "[error] no environment '${ENVIRONMENT}' found under config.environments in $CONFIG" >&2
  exit 1
fi

# Line where the NEXT env block (or EOF) starts, bounding this env's block.
NEXT_LINE=$(awk -v start="$ENV_LINE" 'NR>start && /^    [a-z_]+:[[:space:]]*$/ {print NR; exit}' "$CONFIG")
[ -z "$NEXT_LINE" ] && NEXT_LINE=$(wc -l < "$CONFIG")

# Replace only the value token within [ENV_LINE, NEXT_LINE]; keep indentation
# + any trailing '# comment'.
sed -i -E "${ENV_LINE},${NEXT_LINE} s/^([[:space:]]+${KEY}:[[:space:]]+)(true|false)/\1${VAL}/" "$CONFIG"
echo "[ok] config.environments.${ENVIRONMENT}.components.${KEY} = ${VAL}"
