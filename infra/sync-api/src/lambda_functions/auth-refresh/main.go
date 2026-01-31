package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-jwt/jwt/v5"
)

var (
	smClient *secretsmanager.Client

	licensePublicKey  string
	jwtPrivateKeyName string
	jwtPublicKeyName  string
	rateLimitsTable   string

	logger *slog.Logger
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	smClient = secretsmanager.NewFromConfig(cfg)

	licensePublicKey = os.Getenv("LICENSE_PUBLIC_KEY")
	jwtPrivateKeyName = os.Getenv("JWT_PRIVATE_KEY")
	jwtPublicKeyName = os.Getenv("JWT_PUBLIC_KEY")

	// Init API layer
	api.Init(api.InitParams{
		Logger: logger,
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(authRefreshHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func authRefreshHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Token refresh request received")

	// Parse request body
	var req api.RefreshRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	if req.RefreshToken == "" || req.License == "" {
		logger.Error("Missing required fields", "has_refresh_token", req.RefreshToken != "", "has_license", req.License != "")
		return api.CreateErrorResponse(400, "invalid_request", "Refresh token and license are required")
	}

	logger.Debug("Request validated")

	// Get JWT public key
	logger.Debug("Fetching JWT public key from Secrets Manager")
	result, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &jwtPublicKeyName,
	})
	if err != nil {
		logger.Error("Failed to get JWT public key", "error", err)
		return api.CreateErrorResponse(500, "internal_error", "Failed to validate token")
	}

	// Parse ECDSA public key
	logger.Debug("Parsing JWT public key")
	jwtPubKey, err := api.ParsePublicKey(*result.SecretString)
	if err != nil {
		logger.Error("Failed to parse JWT public key", "error", err)
		return api.CreateErrorResponse(500, "internal_error", "Failed to validate token")
	}

	// Parse and validate refresh token
	logger.Debug("Parsing and validating refresh token")
	var refreshClaims api.RefreshTokenClaims
	token, err := jwt.ParseWithClaims(req.RefreshToken, &refreshClaims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtPubKey, nil
	})

	if err != nil || !token.Valid {
		logger.Error("Invalid or expired refresh token", "error", err)
		return api.CreateErrorResponse(401, "invalid_token", "Invalid or expired refresh token")
	}

	logger.Info("Refresh token validated", "user_email", refreshClaims.Email)

	// Verify token type
	if refreshClaims.Type != "refresh" {
		logger.Error("Invalid token type", "type", refreshClaims.Type, "expected", "refresh")
		return api.CreateErrorResponse(401, "invalid_token", "Token is not a refresh token")
	}

	// Validate license
	logger.Debug("Validating license")
	licenseData, err := api.ValidateLicense(ctx, req.License, licensePublicKey, smClient)
	if err != nil {
		logger.Error("License validation failed", "error", err)
		return api.CreateErrorResponse(400, "invalid_license", "License is invalid")
	}

	logger.Debug("License validated", "license_email", licenseData.Email)

	// Verify email matches
	if licenseData.Email != refreshClaims.Email {
		logger.Error("Email mismatch", "license_email", licenseData.Email, "token_email", refreshClaims.Email)
		return api.CreateErrorResponse(400, "email_mismatch", "License email does not match token")
	}

	// Verify license hash matches
	licenseHash := hashLicense(req.License)
	if licenseHash != refreshClaims.LicenseKeyHash {
		logger.Error("License hash mismatch", "current_hash", licenseHash, "token_hash", refreshClaims.LicenseKeyHash)
		return api.CreateErrorResponse(400, "license_mismatch", "License has changed")
	}

	logger.Debug("License and token match verified")

	// Get JWT private key from Secrets Manager
	logger.Debug("Fetching JWT private key from Secrets Manager")
	result, err = smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &jwtPrivateKeyName,
	})
	if err != nil {
		logger.Error("Failed to get JWT private key", "error", err)
		return api.CreateErrorResponse(500, "internal_error", "Failed to generate tokens")
	}

	// Generate new tokens
	logger.Info("Generating new tokens", "user_email", licenseData.Email)
	accessToken, newRefreshToken, err := api.GenerateTokens(*result.SecretString, licenseData, licenseHash)
	if err != nil {
		logger.Error("Failed to generate tokens", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to generate tokens")
	}

	logger.Info("Tokens refreshed successfully", "user_email", licenseData.Email)

	// Return new tokens
	response := api.RefreshResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    86400, // 24 hours
	}

	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func hashLicense(license string) string {
	hash := sha256.Sum256([]byte(license))
	return fmt.Sprintf("sha256:%x", hash)
}

func main() {
	lambda.Start(handler)
}
