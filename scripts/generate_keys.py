#!/usr/bin/env python3
"""
License Key Generation Script for BreachLine

This script generates:
1. An ECDSA public/private keypair for signing licenses
2. A sample license file for brenton.smorris@gmail.com valid until end of year
"""

import base64
import json
import uuid
from datetime import datetime, timezone
from pathlib import Path

try:
    from cryptography.hazmat.primitives import hashes, serialization
    from cryptography.hazmat.primitives.asymmetric import ec
    from cryptography.hazmat.backends import default_backend
    import jwt
except ImportError:
    print("Required libraries not found. Install with:")
    print("pip install cryptography PyJWT")
    exit(1)


def load_or_generate_keypair(output_dir: Path):
    """Load existing keypair or generate new one if not found"""
    private_key_path = output_dir / "license_private.pem"
    public_key_path = output_dir / "license_public.pem"
    
    # Check if both keys exist
    if private_key_path.exists() and public_key_path.exists():
        print("Loading existing keypair...")
        
        # Load private key
        with open(private_key_path, 'rb') as f:
            private_key = serialization.load_pem_private_key(
                f.read(),
                password=None,
                backend=default_backend()
            )
        print(f"Private key loaded from: {private_key_path}")
        
        # Load public key
        with open(public_key_path, 'rb') as f:
            public_pem_bytes = f.read()
            public_key = serialization.load_pem_public_key(
                public_pem_bytes,
                backend=default_backend()
            )
        print(f"Public key loaded from: {public_key_path}")
        
        return private_key, public_key, public_pem_bytes.decode('utf-8')
    
    # Generate new keypair if not found
    print("Generating new ECDSA keypair...")
    
    # Generate private key using P-256 curve (same as secp256r1)
    private_key = ec.generate_private_key(ec.SECP256R1(), default_backend())
    
    # Get public key
    public_key = private_key.public_key()
    
    # Serialize private key to PEM format
    private_pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=serialization.NoEncryption()
    )
    
    # Serialize public key to PEM format
    public_pem = public_key.public_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PublicFormat.SubjectPublicKeyInfo
    )
    
    # Save keys
    with open(private_key_path, 'wb') as f:
        f.write(private_pem)
    print(f"Private key saved to: {private_key_path}")
    
    with open(public_key_path, 'wb') as f:
        f.write(public_pem)
    print(f"Public key saved to: {public_key_path}")
    
    return private_key, public_key, public_pem.decode('utf-8')


def generate_license(private_key, email: str, output_dir: Path):
    """Generate a license file signed with the private key"""
    print(f"\nGenerating license for {email}...")
    
    # Set license validity period
    now = datetime.now(timezone.utc)
    end_of_year = datetime(now.year, 12, 31, 23, 59, 59, tzinfo=timezone.utc)
    
    # Create JWT payload
    payload = {
        'id': str(uuid.uuid4()),  # Unique license ID
        'email': email,
        'nbf': int(now.timestamp()),  # Not before (start time)
        'exp': int(end_of_year.timestamp()),  # Expiration (end time)
        'iat': int(now.timestamp()),  # Issued at
    }
    
    print(f"License valid from: {now.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    print(f"License valid until: {end_of_year.strftime('%Y-%m-%d %H:%M:%S UTC')}")
    
    # Sign the JWT with ES256 algorithm (ECDSA with SHA-256)
    token = jwt.encode(payload, private_key, algorithm='ES256')
    
    # Base64 encode the token (as expected by the Go application)
    if isinstance(token, str):
        token_bytes = token.encode('utf-8')
    else:
        token_bytes = token
    
    license_content = base64.b64encode(token_bytes).decode('utf-8')
    
    # Save license file
    license_path = output_dir / "breachline.lic"
    with open(license_path, 'w') as f:
        f.write(license_content)
    
    print(f"License saved to: {license_path}")
    
    return license_content


def main():
    # Get script directory
    script_dir = Path(__file__).parent
    
    print("=" * 60)
    print("BreachLine License Key Generator")
    print("=" * 60)
    
    # Load existing keypair or generate new one
    private_key, public_key, public_pem = load_or_generate_keypair(script_dir)
    
    # Generate license for Brenton
    email = "brenton.smorris@gmail.com"
    license_content = generate_license(private_key, email, script_dir)
    
    print("\n" + "=" * 60)
    print("✓ License generation complete!")
    print("=" * 60)
    print("\nNext steps:")
    print("1. If this is your first time, copy the public key from license_public.pem")
    print("   and replace the PublicKey variable in license.go")
    print("2. Use breachline.lic to test the license feature")
    print("3. Keep license_private.pem secure - you'll need it to generate more licenses")
    print("\n⚠️  IMPORTANT: Keep license_private.pem secret and secure!")


if __name__ == "__main__":
    main()
