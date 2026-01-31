package api

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	License      string `json:"license"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type AccessTokenClaims struct {
	Email            string `json:"email"`
	LicenseExpiresAt string `json:"license_expires_at"`
	LicenseKeyHash   string `json:"license_key_hash"`
	Type             string `json:"type"`
	jwt.RegisteredClaims
}

type RefreshTokenClaims struct {
	Email          string `json:"email"`
	LicenseKeyHash string `json:"license_key_hash"`
	Type           string `json:"type"`
	jwt.RegisteredClaims
}

func GenerateTokens(jwtSecretPEM string, licenseData *LicenseData, licenseHash string) (string, string, error) {
	// Parse ECDSA private key from PEM format
	privateKey, err := ParsePrivateKey(jwtSecretPEM)
	if err != nil {
		return "", "", fmt.Errorf("failed to parse private key: %w", err)
	}

	// Generate access token (24 hours)
	accessClaims := AccessTokenClaims{
		Email:            licenseData.Email,
		LicenseExpiresAt: time.Unix(licenseData.ExpiresAt, 0).Format(time.RFC3339),
		LicenseKeyHash:   licenseHash,
		Type:             "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   licenseData.Email,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodES256, accessClaims)
	accessTokenString, err := accessToken.SignedString(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	// Generate refresh token (30 days)
	refreshClaims := RefreshTokenClaims{
		Email:          licenseData.Email,
		LicenseKeyHash: licenseHash,
		Type:           "refresh",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   licenseData.Email,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodES256, refreshClaims)
	refreshTokenString, err := refreshToken.SignedString(privateKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return accessTokenString, refreshTokenString, nil
}

func ParsePublicKey(pemStr string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	ecdsaPub, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not ECDSA")
	}

	return ecdsaPub, nil
}

func ParsePrivateKey(pemStr string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	// Try parsing as PKCS8 first (standard format)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		// Try parsing as EC private key
		key, err = x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
	}

	ecdsaPriv, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not ECDSA")
	}

	return ecdsaPriv, nil
}
