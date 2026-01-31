package api

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-jwt/jwt/v5"
)

type LicenseData struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	ExpiresAt int64  `json:"exp"`
	NotBefore int64  `json:"nbf"`
}

func ValidateLicense(ctx context.Context, licenseB64 string, licensePublicKey string, smClient *secretsmanager.Client) (*LicenseData, error) {
	// Decode base64 license
	decoded, err := base64.StdEncoding.DecodeString(licenseB64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}

	tokenString := string(decoded)

	// Get public key from Secrets Manager
	result, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &licensePublicKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}

	// Parse ECDSA public key
	pubKey, err := ParsePublicKey(*result.SecretString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	// Parse and validate the JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify the signing method is ECDSA
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return pubKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse JWT: %w", err)
	}

	if !token.Valid {
		return nil, errors.New("JWT token is invalid")
	}

	// Extract claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("failed to parse JWT claims")
	}

	// Extract and validate required fields
	var licenseData LicenseData

	// Extract email
	email, emailOk := claims["email"]
	if !emailOk {
		return nil, errors.New("license missing required 'email' claim")
	}
	if emailStr, ok := email.(string); !ok || emailStr == "" {
		return nil, errors.New("license 'email' claim is empty or invalid")
	} else {
		licenseData.Email = emailStr
	}

	// Extract id
	id, idOk := claims["id"]
	if !idOk {
		return nil, errors.New("license missing required 'id' claim")
	}
	if idStr, ok := id.(string); !ok || idStr == "" {
		return nil, errors.New("license 'id' claim is empty or invalid")
	} else {
		licenseData.ID = idStr
	}

	// Extract exp (expiration)
	if exp, ok := claims["exp"]; ok {
		switch v := exp.(type) {
		case float64:
			licenseData.ExpiresAt = int64(v)
		case int64:
			licenseData.ExpiresAt = v
		default:
			return nil, errors.New("invalid 'exp' claim format")
		}
	}

	// Extract nbf (not before)
	if nbf, ok := claims["nbf"]; ok {
		switch v := nbf.(type) {
		case float64:
			licenseData.NotBefore = int64(v)
		case int64:
			licenseData.NotBefore = v
		default:
			return nil, errors.New("invalid 'nbf' claim format")
		}
	}

	// Check if license has expired
	if time.Now().Unix() > licenseData.ExpiresAt {
		return nil, errors.New("license has expired")
	}

	// Check if license is not yet valid
	if time.Now().Unix() < licenseData.NotBefore {
		return nil, errors.New("license is not yet valid")
	}

	return &licenseData, nil
}
