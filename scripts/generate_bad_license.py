#!/usr/bin/env python3
"""
Bad License Generation Script for BreachLine Testing

This script generates invalid licenses for testing purposes:
1. expired.license - License that has already expired
2. notbefore.license - License that is not yet valid
3. wrongsign.license - License signed with wrong key
4. nosign.license - Unsigned license (no signature)
"""

import base64
import json
import uuid
from datetime import datetime, timedelta, timezone
from pathlib import Path

try:
    from cryptography.hazmat.primitives import serialization
    from cryptography.hazmat.primitives.asymmetric import ec
    from cryptography.hazmat.backends import default_backend
    import jwt
except ImportError:
    print("Required libraries not found. Install with:")
    print("pip install cryptography PyJWT")
    exit(1)


def load_existing_keys(script_dir: Path):
    """Load existing keypair"""
    private_key_path = script_dir / "license_private.pem"
    public_key_path = script_dir / "license_public.pem"
    
    if not (private_key_path.exists() and public_key_path.exists()):
        print("ERROR: Keys not found. Please run generate_keys.py first.")
        exit(1)
    
    print("Loading existing keypair...")
    
    # Load private key
    with open(private_key_path, 'rb') as f:
        private_key = serialization.load_pem_private_key(
            f.read(),
            password=None,
            backend=default_backend()
        )
    
    return private_key


def generate_expired_license(private_key, email: str, output_dir: Path):
    """Generate a license that has already expired"""
    print("\n1. Generating expired license...")
    
    # Set dates: notbefore = 1 year ago, notafter = yesterday
    now = datetime.now(timezone.utc)
    one_year_ago = now - timedelta(days=365)
    yesterday = now - timedelta(days=1)
    
    payload = {
        'id': str(uuid.uuid4()),  # Unique license ID
        'email': email,
        'nbf': int(one_year_ago.timestamp()),
        'exp': int(yesterday.timestamp()),
        'iat': int(one_year_ago.timestamp()),
    }
    
    print(f"   Not before: {one_year_ago.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    print(f"   Expired at: {yesterday.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    
    # Sign the JWT
    token = jwt.encode(payload, private_key, algorithm='ES256')
    if isinstance(token, str):
        token_bytes = token.encode('utf-8')
    else:
        token_bytes = token
    
    license_content = base64.b64encode(token_bytes).decode('utf-8')
    
    # Save license file
    license_path = output_dir / "expired.license"
    with open(license_path, 'w') as f:
        f.write(license_content)
    
    print(f"   Saved to: {license_path}")


def generate_notbefore_license(private_key, email: str, output_dir: Path):
    """Generate a license that is not yet valid"""
    print("\n2. Generating not-yet-valid license...")
    
    # Set dates: notbefore = 1 hour from now, notafter = tomorrow
    now = datetime.now(timezone.utc)
    one_hour_later = now + timedelta(hours=1)
    tomorrow = now + timedelta(days=1)
    
    payload = {
        'id': str(uuid.uuid4()),  # Unique license ID
        'email': email,
        'nbf': int(one_hour_later.timestamp()),
        'exp': int(tomorrow.timestamp()),
        'iat': int(now.timestamp()),
    }
    
    print(f"   Not before: {one_hour_later.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    print(f"   Expires at: {tomorrow.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    
    # Sign the JWT
    token = jwt.encode(payload, private_key, algorithm='ES256')
    if isinstance(token, str):
        token_bytes = token.encode('utf-8')
    else:
        token_bytes = token
    
    license_content = base64.b64encode(token_bytes).decode('utf-8')
    
    # Save license file
    license_path = output_dir / "notbefore.license"
    with open(license_path, 'w') as f:
        f.write(license_content)
    
    print(f"   Saved to: {license_path}")


def generate_wrongsign_license(email: str, output_dir: Path):
    """Generate a license signed with a different key"""
    print("\n3. Generating wrong-signature license...")
    
    # Generate a throwaway key in memory (not saved)
    print("   Generating throwaway key for wrong signature...")
    throwaway_key = ec.generate_private_key(ec.SECP256R1(), default_backend())
    
    # Set valid dates
    now = datetime.now(timezone.utc)
    end_of_year = datetime(now.year, 12, 31, 23, 59, 59, tzinfo=timezone.utc)
    
    payload = {
        'id': str(uuid.uuid4()),  # Unique license ID
        'email': email,
        'nbf': int(now.timestamp()),
        'exp': int(end_of_year.timestamp()),
        'iat': int(now.timestamp()),
    }
    
    print(f"   Not before: {now.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    print(f"   Expires at: {end_of_year.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    
    # Sign the JWT with the WRONG key
    token = jwt.encode(payload, throwaway_key, algorithm='ES256')
    if isinstance(token, str):
        token_bytes = token.encode('utf-8')
    else:
        token_bytes = token
    
    license_content = base64.b64encode(token_bytes).decode('utf-8')
    
    # Save license file
    license_path = output_dir / "wrongsign.license"
    with open(license_path, 'w') as f:
        f.write(license_content)
    
    print(f"   Saved to: {license_path}")


def generate_nosign_license(email: str, output_dir: Path):
    """Generate an unsigned JWT"""
    print("\n4. Generating unsigned license...")
    
    # Set valid dates
    now = datetime.now(timezone.utc)
    end_of_year = datetime(now.year, 12, 31, 23, 59, 59, tzinfo=timezone.utc)
    
    payload = {
        'id': str(uuid.uuid4()),  # Unique license ID
        'email': email,
        'nbf': int(now.timestamp()),
        'exp': int(end_of_year.timestamp()),
        'iat': int(now.timestamp()),
    }
    
    print(f"   Not before: {now.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    print(f"   Expires at: {end_of_year.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    
    # Create unsigned JWT (algorithm 'none')
    token = jwt.encode(payload, None, algorithm='none')
    if isinstance(token, str):
        token_bytes = token.encode('utf-8')
    else:
        token_bytes = token
    
    license_content = base64.b64encode(token_bytes).decode('utf-8')
    
    # Save license file
    license_path = output_dir / "nosign.license"
    with open(license_path, 'w') as f:
        f.write(license_content)
    
    print(f"   Saved to: {license_path}")


def main():
    # Get script directory
    script_dir = Path(__file__).parent
    
    print("=" * 60)
    print("BreachLine Bad License Generator (for testing)")
    print("=" * 60)
    
    # Load existing keys
    private_key = load_existing_keys(script_dir)
    
    # Email for all test licenses
    email = "brenton.smorris@gmail.com"
    
    # Generate all bad licenses
    generate_expired_license(private_key, email, script_dir)
    generate_notbefore_license(private_key, email, script_dir)
    generate_wrongsign_license(email, script_dir)
    generate_nosign_license(email, script_dir)
    
    print("\n" + "=" * 60)
    print("✓ Bad license generation complete!")
    print("=" * 60)
    print("\nGenerated test licenses:")
    print("1. expired.license     - License that has already expired")
    print("2. notbefore.license   - License not yet valid (valid in 1 hour)")
    print("3. wrongsign.license   - License signed with wrong key")
    print("4. nosign.license      - Unsigned license")
    print("\nUse these to test that the license validation is working correctly.")


if __name__ == "__main__":
    main()
