#!/usr/bin/env python3
"""Generate a signed BreachLine license file.

Signs an ES256 JWT with the license signing key held in `secrets.vault.yml`
(`vault.environments.<env>.license.signing_private_key`) and writes it
base64-encoded, matching the format produced by the payment-api
license-generator lambda and expected by `application/app/license.go`.

The private key is only ever held in memory - it is never logged or written
to disk.

By default the environment is auto-detected: the script derives the public key
for each environment's signing key and picks the one matching the public key
embedded in `application/app/license.go`, so the generated license is accepted
by a locally built app.

Examples:
    python3 scripts/generate_license.py --email you@example.com --days 30
    python3 scripts/generate_license.py --email you@example.com --days 30 \\
        --env prod --output /tmp/breachline.lic
"""

import argparse
import base64
import logging
import re
import subprocess
import sys
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path

try:
    import yaml
    import jwt
    from cryptography.hazmat.backends import default_backend
    from cryptography.hazmat.primitives import serialization
except ModuleNotFoundError as exc:
    sys.exit(f"missing dependency ({exc.name}); run `make venv` or "
             f"`pip install -r scripts/requirements.txt`")

REPO = Path(__file__).resolve().parents[1]
VAULT_FILE = REPO / "secrets.vault.yml"
VAULT_PW = REPO / ".vault-password-file"
APP_LICENSE_GO = REPO / "application" / "app" / "license.go"

ENVIRONMENTS = ("dev", "prod")

logger = logging.getLogger("generate-license")


def load_vault() -> dict:
    """Decrypt secrets.vault.yml and return the `vault:` block."""
    if not VAULT_FILE.exists():
        sys.exit(f"{VAULT_FILE} not found - run `make init`")
    if not VAULT_PW.exists():
        sys.exit(f"{VAULT_PW} not found - restore from 1Password")

    logger.debug("decrypting %s", VAULT_FILE.name)
    proc = subprocess.run(
        ["ansible-vault", "view", "--vault-password-file", str(VAULT_PW),
         str(VAULT_FILE)],
        capture_output=True, text=True, check=False,
    )
    if proc.returncode != 0:
        sys.exit(f"ansible-vault view failed:\n{proc.stderr}")

    data = yaml.safe_load(proc.stdout)
    if not isinstance(data, dict) or "vault" not in data:
        sys.exit(f"{VAULT_FILE} must have a top-level `vault:` key")
    return data["vault"]


def signing_key_for(vault: dict, env: str):
    """Load the ECDSA signing private key for `env` from the vault."""
    pem = (((vault.get("environments") or {}).get(env) or {})
           .get("license") or {}).get("signing_private_key")
    if not pem:
        sys.exit(f"vault.environments.{env}.license.signing_private_key "
                 f"is missing - run `make init`")
    try:
        return serialization.load_pem_private_key(
            pem.encode("utf-8"), password=None, backend=default_backend())
    except ValueError as exc:
        sys.exit(f"could not parse signing key for {env}: {exc}")


def public_pem(private_key) -> str:
    """Return the PEM public key derived from a private key, normalised."""
    pem = private_key.public_key().public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    ).decode("utf-8")
    return "".join(pem.split())


def app_public_pem() -> str | None:
    """Extract the public key embedded in application/app/license.go."""
    if not APP_LICENSE_GO.exists():
        logger.warning("%s not found - cannot verify key match",
                       APP_LICENSE_GO)
        return None
    match = re.search(
        r"-----BEGIN PUBLIC KEY-----.*?-----END PUBLIC KEY-----",
        APP_LICENSE_GO.read_text(), re.DOTALL)
    if not match:
        logger.warning("no public key found in %s", APP_LICENSE_GO.name)
        return None
    return "".join(match.group(0).split())


def detect_env(vault: dict) -> str:
    """Pick the environment whose signing key matches the app's public key."""
    embedded = app_public_pem()
    if embedded is None:
        logger.warning("falling back to env=prod")
        return "prod"

    for env in ENVIRONMENTS:
        if public_pem(signing_key_for(vault, env)) == embedded:
            logger.info("auto-detected env=%s (matches key in %s)",
                        env, APP_LICENSE_GO.name)
            return env

    sys.exit("no vault signing key matches the public key embedded in "
             f"{APP_LICENSE_GO.name}; pass --env explicitly")


def build_license(private_key, email: str, days: int) -> tuple[str, dict]:
    """Sign an ES256 JWT license and return (base64 content, claims)."""
    now = datetime.now(timezone.utc)
    expiration = now + timedelta(days=days)

    claims = {
        "id": str(uuid.uuid4()),
        "email": email,
        "nbf": int(now.timestamp()),
        "exp": int(expiration.timestamp()),
        "iat": int(now.timestamp()),
    }

    logger.info("license id %s", claims["id"])
    logger.info("valid from %s", now.strftime("%Y-%m-%d %H:%M:%S UTC"))
    logger.info("valid until %s", expiration.strftime("%Y-%m-%d %H:%M:%S UTC"))

    token = jwt.encode(claims, private_key, algorithm="ES256")
    if isinstance(token, bytes):
        token = token.decode("utf-8")

    return base64.b64encode(token.encode("utf-8")).decode("utf-8"), claims


def verify(license_content: str, private_key) -> None:
    """Re-verify the generated license against the matching public key."""
    token = base64.b64decode(license_content).decode("utf-8")
    jwt.decode(token, private_key.public_key(), algorithms=["ES256"])
    logger.info("signature and time claims verified")


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Generate a signed BreachLine license file.")
    parser.add_argument("--email", required=True,
                        help="email address the license is issued to")
    parser.add_argument("--days", type=int, default=30,
                        help="validity period in days (default: 30)")
    parser.add_argument("--env", choices=ENVIRONMENTS,
                        help="signing environment (default: auto-detect from "
                             "the key embedded in the application)")
    parser.add_argument("--output", type=Path,
                        help="output path (default: <email-local-part>.lic in "
                             "the current directory)")
    parser.add_argument("--verbose", action="store_true",
                        help="enable debug logging")
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(levelname)s %(message)s")

    if args.days <= 0:
        sys.exit("--days must be greater than 0")

    vault = load_vault()
    env = args.env or detect_env(vault)
    private_key = signing_key_for(vault, env)
    logger.info("signing with the %s license key", env)

    license_content, claims = build_license(private_key, args.email, args.days)
    verify(license_content, private_key)

    output = args.output or Path(f"{args.email.split('@')[0]}.lic")
    output.write_text(license_content)
    output.chmod(0o600)
    logger.info("wrote %s (%d bytes)", output, len(license_content))


if __name__ == "__main__":
    main()
