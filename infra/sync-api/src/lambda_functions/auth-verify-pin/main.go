package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"golang.org/x/crypto/bcrypt"
)

type UserSubscription struct {
	Email          string `dynamodbav:"email"`
	WorkspaceLimit int    `dynamodbav:"workspace_limit"`
	SeatCount      int    `dynamodbav:"seat_count"`
	CreatedAt      string `dynamodbav:"created_at"`
	UpdatedAt      string `dynamodbav:"updated_at"`
}

var (
	smClient *secretsmanager.Client

	pinsTable           string
	subscriptionsTable  string
	rateLimitsTable     string
	licensePublicKey    string
	jwtPrivateKeyName   string
	rateLimitMiddleware *api.RateLimitMiddleware

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

	pinsTable = os.Getenv("PINS_TABLE")
	subscriptionsTable = os.Getenv("USER_SUBSCRIPTIONS_TABLE")
	licensePublicKey = os.Getenv("LICENSE_PUBLIC_KEY")
	jwtPrivateKeyName = os.Getenv("JWT_PRIVATE_KEY")

	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")

	// Init API layer
	api.Init(api.InitParams{
		Logger:             logger,
		PinsTable:          pinsTable,
		SubscriptionsTable: subscriptionsTable,
	})

	// Initialize rate limits
	api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)

	// Create rate limit middleware
	rateLimitMiddleware = api.NewRateLimitMiddleware(api.GetDefaultConfig(), logger)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("PIN verification request received")

	// Parse request body
	var req api.VerifyPINRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	if req.License == "" || req.PIN == "" {
		logger.Error("Missing required fields", "has_license", req.License != "", "has_pin", req.PIN != "")
		return api.CreateErrorResponse(400, "invalid_request", "License and PIN are required")
	}

	// Validate license
	logger.Debug("Validating license")
	licenseData, err := api.ValidateLicense(ctx, req.License, licensePublicKey, smClient)
	if err != nil {
		logger.Error("License validation failed", "error", err)
		return api.CreateErrorResponse(400, "invalid_license", "License is invalid")
	}
	logger.Info("License validated", "user_email", licenseData.Email)

	// Check email-based rate limit (shared with request-pin - prevents brute force attacks)
	logger.Debug("Checking email-based rate limit", "user_email", licenseData.Email)
	allowed, headers, rateLimitInfo, err := rateLimitMiddleware.CheckAuthRateLimit(ctx, licenseData.Email, "/auth/verify-pin")
	if err != nil {
		logger.Error("Failed to check auth rate limit", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to process request")
	}
	if !allowed {
		logger.Warn("Auth rate limit exceeded", "user_email", licenseData.Email)
		return rateLimitMiddleware.CreateRateLimitErrorResponse(headers, rateLimitInfo), nil
	}

	// Check if license has expired
	if time.Now().Unix() > licenseData.ExpiresAt {
		logger.Error("License expired", "user_email", licenseData.Email, "expires_at", licenseData.ExpiresAt)
		return api.CreateErrorResponse(400, "license_expired", "License has expired")
	}

	// Check if license is not yet valid
	if time.Now().Unix() < licenseData.NotBefore {
		logger.Error("License not yet valid", "user_email", licenseData.Email, "not_before", licenseData.NotBefore)
		return api.CreateErrorResponse(400, "license_not_valid_yet", "License is not yet valid")
	}

	// Get all valid PIN records from DynamoDB
	logger.Debug("Fetching valid PIN records", "user_email", licenseData.Email)
	validPins, err := api.GetValidPINs(ctx, licenseData.Email)
	if err != nil {
		logger.Error("Failed to get valid PIN records", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(400, "invalid_pin", "Invalid or expired PIN")
	}

	// Try to verify the PIN against any valid PIN record
	logger.Debug("Verifying PIN against valid records", "user_email", licenseData.Email, "valid_pin_count", len(validPins))
	var matchedPin *api.PIN
	licenseHash := hashLicense(req.License)

	for i := range validPins {
		pin := &validPins[i]

		// Verify license hash matches first (cheaper check)
		if licenseHash != pin.LicenseKeyHash {
			continue
		}

		// Verify PIN hash
		if err := bcrypt.CompareHashAndPassword([]byte(pin.PinHash), []byte(req.PIN)); err == nil {
			matchedPin = pin
			break
		}
	}

	if matchedPin == nil {
		logger.Error("No matching PIN found", "user_email", licenseData.Email)
		return api.CreateErrorResponse(400, "invalid_pin", "Invalid PIN")
	}

	logger.Info("PIN verified successfully", "user_email", licenseData.Email, "matched_pin_hash_prefix", matchedPin.PinHash[:8]+"...")

	// Initialize or get user subscription
	logger.Debug("Ensuring user subscription", "user_email", licenseData.Email)
	if err := api.EnsureUserSubscription(ctx, licenseData); err != nil {
		logger.Error("Failed to ensure user subscription", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to initialize user subscription")
	}

	// Get JWT private key from Secrets Manager
	logger.Debug("Fetching JWT private key from Secrets Manager")
	result, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &jwtPrivateKeyName,
	})
	if err != nil {
		logger.Error("Failed to get JWT private key", "error", err)
		return api.CreateErrorResponse(500, "internal_error", "Failed to generate tokens")
	}

	// Generate JWT tokens
	logger.Info("Generating JWT tokens", "user_email", licenseData.Email)
	accessToken, refreshToken, err := api.GenerateTokens(*result.SecretString, licenseData, licenseHash)
	if err != nil {
		logger.Error("Failed to generate tokens", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to generate tokens")
	}
	logger.Info("Tokens generated successfully", "user_email", licenseData.Email)

	// Delete the specific PIN that was used
	logger.Debug("Deleting used PIN", "user_email", licenseData.Email, "pin_hash_prefix", matchedPin.PinHash[:10]+"...")
	if err := api.DeletePIN(ctx, licenseData.Email, matchedPin.PinHash); err != nil {
		logger.Error("Failed to delete PIN", "error", err, "user_email", licenseData.Email, "pin_hash_prefix", matchedPin.PinHash[:8]+"...")
		// Don't fail the request
	} else {
		logger.Debug("PIN deleted successfully", "user_email", licenseData.Email, "pin_hash_prefix", matchedPin.PinHash[:8]+"...")
	}

	// Return tokens
	response := api.VerifyPINResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
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
