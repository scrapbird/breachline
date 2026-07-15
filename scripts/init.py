#!/usr/bin/env python3
"""scripts/init.py - Bootstrap OR repair config.yml + secrets.vault.yml.

Idempotent. Safe to re-run any time.

What gets created/repaired:
  - .vault-password-file:  created (32 random bytes, base64) if missing.
                           BACK THIS UP in 1Password - it decrypts the vault.
  - config.yml:            created with default content if missing. Existing
                           files are NOT modified - edit with `make edit-config`.
  - secrets.vault.yml:     created with placeholders if missing. If it exists:
                           decrypt, add any missing key, re-encrypt.

Per environment (dev, prod) the vault carries:
  Manual-fill (external creds; init writes "" placeholders, you set them):
    - vault.environments.<env>.stripe.api_key       (sk_test_ for dev, sk_live_ for prod)
    - vault.environments.<env>.stripe.webhook_secret (whsec_...)
  Auto-generated (ECDSA P-256 keypairs, minted when missing/blank):
    - vault.environments.<env>.license.{signing_private_key, public_key}
    - vault.environments.<env>.jwt.{private_key, public_key}

The license public_key MUST match its signing_private_key, so init generates
them as a pair. dev gets its own license keypair by default; to accept
prod-signed licenses in dev, copy prod's whole `license:` block into dev.
"""

import os
import secrets
import subprocess
import sys
import tempfile
from pathlib import Path

try:
    import yaml
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric import ec
except ModuleNotFoundError:
    sys.exit("deps missing; run `make venv` (needs pyyaml + cryptography)")

REPO = Path(__file__).resolve().parents[1]
VAULT_FILE = REPO / "secrets.vault.yml"
CONFIG_FILE = REPO / "config.yml"
VAULT_PW = REPO / ".vault-password-file"

ENVIRONMENTS = ("dev", "prod")


def run(cmd):
    r = subprocess.run(cmd, capture_output=True, text=True, check=False)
    if r.returncode != 0:
        sys.exit(f"[error] {cmd[0]} failed:\n{r.stderr or r.stdout}")
    return r.stdout


def get_path(d, *path):
    cur = d
    for k in path:
        if not isinstance(cur, dict) or k not in cur:
            return None
        cur = cur[k]
    return cur


def set_path(d, value, *path):
    cur = d
    for k in path[:-1]:
        cur = cur.setdefault(k, {})
    cur[path[-1]] = value


# ============================================================================
# YAML dumping with literal-block scalars for the multi-line PEM keys.
# ============================================================================

class _LiteralStr(str):
    pass


def _literal_repr(dumper, data):
    return dumper.represent_scalar("tag:yaml.org,2002:str", data, style="|")


yaml.add_representer(_LiteralStr, _literal_repr)


def _wrap_multiline(node):
    if isinstance(node, dict):
        return {k: _wrap_multiline(v) for k, v in node.items()}
    if isinstance(node, list):
        return [_wrap_multiline(x) for x in node]
    if isinstance(node, str) and "\n" in node:
        return _LiteralStr(node)
    return node


# ============================================================================
# ECDSA P-256 keypair generation
# ============================================================================

def gen_ec_keypair():
    """Return (private_pem, public_pem) for a fresh ECDSA P-256 keypair."""
    key = ec.generate_private_key(ec.SECP256R1())
    priv = key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption(),
    ).decode()
    pub = key.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    ).decode()
    return priv, pub


def _blank(v):
    return v is None or (isinstance(v, str) and not v.strip())


# ============================================================================
# Vault repair / generation
# ============================================================================

VAULT_HEADER = """\
---
# ============================================================================
# BreachLine secrets - ansible-vault encrypted, committed to git.
# Edit via:  make edit-secrets
# Add a blanked/missing placeholder back:  make init
# Render:    make render ENV=<env>
# Non-secret config lives in config.yml at the repo root.
# Both files share a top-level namespace (vault: here, config: there) and an
# environments.<env> block so dev/prod carry different secrets.
# ============================================================================
"""


def repair_vault(vault: dict) -> bool:
    """Ensure every env has stripe placeholders + license/jwt keypairs.
    Returns True if anything changed."""
    changed = False
    for env in ENVIRONMENTS:
        # External creds -> placeholder only.
        for path in (("stripe", "api_key"), ("stripe", "webhook_secret")):
            full = ("environments", env, *path)
            if get_path(vault, *full) is None:
                set_path(vault, "", *full)
                changed = True
                print(f"  [add] vault.environments.{env}.{'.'.join(path)} "
                      f"placeholder (FILL IN via make edit-secrets)")

        # license keypair (signing_private_key + matching public_key).
        priv = get_path(vault, "environments", env, "license", "signing_private_key")
        pub = get_path(vault, "environments", env, "license", "public_key")
        if _blank(priv) or _blank(pub):
            priv, pub = gen_ec_keypair()
            set_path(vault, priv, "environments", env, "license", "signing_private_key")
            set_path(vault, pub, "environments", env, "license", "public_key")
            changed = True
            print(f"  [gen] vault.environments.{env}.license.* (ECDSA P-256 keypair)")

        # jwt keypair.
        jpriv = get_path(vault, "environments", env, "jwt", "private_key")
        jpub = get_path(vault, "environments", env, "jwt", "public_key")
        if _blank(jpriv) or _blank(jpub):
            jpriv, jpub = gen_ec_keypair()
            set_path(vault, jpriv, "environments", env, "jwt", "private_key")
            set_path(vault, jpub, "environments", env, "jwt", "public_key")
            changed = True
            print(f"  [gen] vault.environments.{env}.jwt.* (ECDSA P-256 keypair)")
    return changed


def vault_decrypt() -> dict:
    out = run(["ansible-vault", "view",
               "--vault-password-file", str(VAULT_PW), str(VAULT_FILE)])
    data = yaml.safe_load(out)
    if not isinstance(data, dict) or "vault" not in data:
        sys.exit(f"{VAULT_FILE} did not decode to a dict with top-level `vault:` key")
    return data["vault"]


def vault_write(vault_dict: dict) -> None:
    payload = {"vault": _wrap_multiline(vault_dict)}
    with tempfile.NamedTemporaryFile("w", suffix=".yml", delete=False) as f:
        f.write(VAULT_HEADER)
        yaml.dump(payload, f, default_flow_style=False, sort_keys=False,
                  indent=2, width=10000)
        tmp = f.name
    try:
        run(["ansible-vault", "encrypt",
             "--vault-password-file", str(VAULT_PW),
             "--output", str(VAULT_FILE), tmp])
    finally:
        os.remove(tmp)


# ============================================================================
# config.yml default content (only used when the file is missing)
# ============================================================================

CONFIG_DEFAULT = (REPO / "scripts" / "config.default.yml")


def main():
    # 1. .vault-password-file
    if not VAULT_PW.exists():
        VAULT_PW.write_text(secrets.token_urlsafe(32) + "\n")
        VAULT_PW.chmod(0o600)
        print(f"[ok] generated {VAULT_PW.relative_to(REPO)} (BACK THIS UP in 1Password)")
    else:
        print(f"[skip] {VAULT_PW.relative_to(REPO)} already exists")

    # 2. config.yml - create if missing; never modify if present.
    if not CONFIG_FILE.exists():
        if CONFIG_DEFAULT.exists():
            CONFIG_FILE.write_text(CONFIG_DEFAULT.read_text())
            print(f"[ok] generated {CONFIG_FILE.relative_to(REPO)} (review + edit)")
        else:
            sys.exit(f"[error] {CONFIG_FILE} missing and no template at {CONFIG_DEFAULT}")
    else:
        print(f"[skip] {CONFIG_FILE.relative_to(REPO)} exists - edit with 'make edit-config'")

    # 3. secrets.vault.yml - create if missing, else repair.
    if not VAULT_FILE.exists():
        print(f"[gen] {VAULT_FILE.relative_to(REPO)} (initial bootstrap)")
        vault: dict = {}
        repair_vault(vault)
        vault_write(vault)
        print(f"[ok] generated encrypted {VAULT_FILE.relative_to(REPO)}")
    else:
        vault = vault_decrypt()
        if repair_vault(vault):
            vault_write(vault)
            print(f"[ok] repaired {VAULT_FILE.relative_to(REPO)}")
        else:
            print(f"[skip] {VAULT_FILE.relative_to(REPO)} fully populated")

    print()
    print("Next steps:")
    print("  make edit-secrets   # set vault.environments.<env>.stripe.{api_key,webhook_secret}")
    print("  make render ENV=dev # generate secrets.env + infra/*/dev.tfvars")
    print("  make deploy ENV=dev # build lambdas + terraform apply the dev stack")


if __name__ == "__main__":
    main()
