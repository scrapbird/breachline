# JWT private key for signing tokens
resource "aws_secretsmanager_secret" "jwt_private_key" {
  name        = "breachline-sync-jwt-private-key${local.suffix}"
  description = "ECDSA private key for signing JWT tokens"

  recovery_window_in_days = local.secret_recovery_days

  tags = {
    Name = "breachline-sync-jwt-private-key${local.suffix}"
  }
}

# JWT public key for verifying tokens
resource "aws_secretsmanager_secret" "jwt_public_key" {
  name        = "breachline-sync-jwt-public-key${local.suffix}"
  description = "ECDSA public key for verifying JWT tokens"

  recovery_window_in_days = local.secret_recovery_days

  tags = {
    Name = "breachline-sync-jwt-public-key${local.suffix}"
  }
}

# License public key for validation
resource "aws_secretsmanager_secret" "license_public_key" {
  name        = "breachline-sync-license-public-key${local.suffix}"
  description = "ECDSA public key for validating license JWT signatures"

  recovery_window_in_days = local.secret_recovery_days

  tags = {
    Name = "breachline-sync-license-public-key${local.suffix}"
  }
}

# Secret values are populated from the ansible-vault (via TF_VAR_* sourced from
# secrets.env). The JWT keypair signs/verifies session tokens; the license
# public key validates license JWTs issued by payment-api's signing key.
resource "aws_secretsmanager_secret_version" "jwt_private_key" {
  secret_id     = aws_secretsmanager_secret.jwt_private_key.id
  secret_string = var.jwt_private_key
}

resource "aws_secretsmanager_secret_version" "jwt_public_key" {
  secret_id     = aws_secretsmanager_secret.jwt_public_key.id
  secret_string = var.jwt_public_key
}

resource "aws_secretsmanager_secret_version" "license_public_key" {
  secret_id     = aws_secretsmanager_secret.license_public_key.id
  secret_string = var.license_public_key
}
