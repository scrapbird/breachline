# JWT private key for signing tokens
resource "aws_secretsmanager_secret" "jwt_private_key" {
  name        = "breachline-sync-jwt-private-key"
  description = "ECDSA private key for signing JWT tokens"

  tags = {
    Name = "breachline-sync-jwt-private-key"
  }
}

# JWT public key for verifying tokens
resource "aws_secretsmanager_secret" "jwt_public_key" {
  name        = "breachline-sync-jwt-public-key"
  description = "ECDSA public key for verifying JWT tokens"

  tags = {
    Name = "breachline-sync-jwt-public-key"
  }
}

# License public key for validation
resource "aws_secretsmanager_secret" "license_public_key" {
  name        = "breachline-sync-license-public-key"
  description = "ECDSA public key for validating license JWT signatures"

  tags = {
    Name = "breachline-sync-license-public-key"
  }
}

resource "aws_secretsmanager_secret_version" "license_public_key" {
  secret_id     = aws_secretsmanager_secret.license_public_key.id
  secret_string = var.license_public_key
}
