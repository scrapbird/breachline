#!/usr/bin/env bash
# Wrapper around `ansible-vault edit` that only writes the encrypted file if
# the plaintext actually changed. Prevents stale editor sessions from silently
# overwriting the vault with old content.
set -euo pipefail

REPO="$(cd "$(dirname "$0")/.." && pwd)"
VAULT="$REPO/secrets.vault.yml"
VAULT_PW="$REPO/.vault-password-file"

if [ ! -f "$VAULT" ]; then
  echo "[error] $VAULT not found - run 'make init'" >&2
  exit 1
fi
if [ ! -f "$VAULT_PW" ]; then
  echo "[error] $VAULT_PW not found - restore from 1Password" >&2
  exit 1
fi

BEFORE=$(ansible-vault view --vault-password-file "$VAULT_PW" "$VAULT")

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

echo "$BEFORE" > "$TMP"
${EDITOR:-vi} "$TMP"

AFTER=$(cat "$TMP")

if [ "$BEFORE" = "$AFTER" ]; then
  echo "[skip] no changes made - vault unchanged"
  exit 0
fi

ansible-vault encrypt \
  --vault-password-file "$VAULT_PW" \
  --output "$VAULT" \
  "$TMP"

echo "[ok] vault updated"
