package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	sestypes "github.com/aws/aws-sdk-go-v2/service/ses/types"
	"github.com/scrapbird/breachline/infra/sync-api/src/api"
	"golang.org/x/crypto/bcrypt"
)

var (
	sesClient *ses.Client
	smClient  *secretsmanager.Client

	pinsTable           string
	rateLimitsTable     string
	licensePublicKey    string
	sesFromEmail        string
	sesConfigSet        string
	pinTTLHours         int
	pinRateLimitSec     int
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

	sesClient = ses.NewFromConfig(cfg)
	smClient = secretsmanager.NewFromConfig(cfg)

	pinsTable = os.Getenv("PINS_TABLE")
	licensePublicKey = os.Getenv("LICENSE_PUBLIC_KEY")
	sesFromEmail = os.Getenv("SES_FROM_EMAIL")
	sesConfigSet = os.Getenv("SES_CONFIGURATION_SET")

	ttlStr := os.Getenv("PIN_TTL_HOURS")
	pinTTLHours, err = strconv.Atoi(ttlStr)
	if err != nil {
		pinTTLHours = 12 // default
	}

	rateLimitStr := os.Getenv("PIN_RATE_LIMIT_SECONDS")
	pinRateLimitSec, err = strconv.Atoi(rateLimitStr)
	if err != nil {
		pinRateLimitSec = 300 // default: 5 minutes
	}

	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")

	// Init API layer
	api.Init(api.InitParams{
		Logger:             logger,
		PinsTable:          pinsTable,
		SubscriptionsTable: os.Getenv("USER_SUBSCRIPTIONS_TABLE"),
	})

	// Initialize rate limits
	api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)

	// Create rate limit middleware
	rateLimitMiddleware = api.NewRateLimitMiddleware(api.GetDefaultConfig(), logger)
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("PIN request received")

	// Parse request body
	var req api.RequestPINRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	if req.License == "" {
		logger.Error("Missing license")
		return api.CreateErrorResponse(400, "invalid_request", "License is required")
	}

	// Decode and validate license
	logger.Debug("Validating license")
	licenseData, err := api.ValidateLicense(ctx, req.License, licensePublicKey, smClient)
	if err != nil {
		logger.Error("License validation failed", "error", err)
		return api.CreateErrorResponse(400, "invalid_license", "License is invalid or could not extract email")
	}
	logger.Info("License validated", "user_email", licenseData.Email)

	// Check that the user has a valid sync subscription
	logger.Debug("Checking user subscription", "user_email", licenseData.Email)
	subscription, err := api.GetUserSubscription(ctx, licenseData.Email)
	if err != nil {
		logger.Error("Failed to get user subscription", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(403, "subscription_required", "Valid sync subscription required")
	}
	logger.Info("User subscription validated", "user_email", licenseData.Email, "workspace_limit", subscription.WorkspaceLimit)

	// Check email-based rate limit first (prevents abuse even after PIN deletion)
	logger.Debug("Checking email-based rate limit", "user_email", licenseData.Email)
	allowed, headers, rateLimitInfo, err := rateLimitMiddleware.CheckAuthRateLimit(ctx, licenseData.Email, "/auth/request-pin")
	if err != nil {
		logger.Error("Failed to check auth rate limit", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to process request")
	}
	if !allowed {
		logger.Warn("Auth rate limit exceeded", "user_email", licenseData.Email)
		return rateLimitMiddleware.CreateRateLimitErrorResponse(headers, rateLimitInfo), nil
	}

	// Check PIN cooldown - ensure no PINs were created recently (5 minute cooldown between PIN creations)
	logger.Debug("Checking PIN cooldown", "user_email", licenseData.Email, "rate_limit_seconds", pinRateLimitSec)
	isOnCooldown, err := checkPINCooldown(ctx, licenseData.Email)
	if err != nil {
		logger.Error("Failed to check PIN cooldown", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to process request")
	}
	if isOnCooldown {
		logger.Warn("PIN cooldown active", "user_email", licenseData.Email, "rate_limit_seconds", pinRateLimitSec)
		return api.CreateErrorResponse(429, "rate_limited", fmt.Sprintf("A PIN was recently created. Please wait %d minutes before requesting another PIN.", pinRateLimitSec/60))
	}

	// Generate 6-digit PIN
	logger.Debug("Generating PIN", "user_email", licenseData.Email)
	randomPin, err := generatePIN()
	if err != nil {
		logger.Error("Failed to generate PIN", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to generate PIN")
	}

	// Hash the PIN for storage
	logger.Debug("Hashing PIN", "user_email", licenseData.Email)
	pinHash, err := bcrypt.GenerateFromPassword([]byte(randomPin), bcrypt.DefaultCost)
	if err != nil {
		logger.Error("Failed to hash PIN", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to process PIN")
	}

	// Calculate expiration time
	now := time.Now()
	expiresAt := now.Add(time.Duration(pinTTLHours) * time.Hour)
	logger.Debug("PIN expiration calculated", "expires_at", expiresAt, "ttl_hours", pinTTLHours)

	// Hash the license for tracking
	licenseHash := hashLicense(req.License)

	// Store PIN in DynamoDB
	pin := api.PIN{
		Email:          licenseData.Email,
		PinHash:        string(pinHash),
		LicenseKeyHash: licenseHash,
		ExpiresAt:      expiresAt.Unix(),
		CreatedAt:      now.Format(time.RFC3339),
	}

	err = api.CreatePin(ctx, pin)
	if err != nil {
		logger.Error("Failed to store PIN", "error", err, "user_email", licenseData.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to store PIN")
	}
	logger.Info("PIN stored successfully", "user_email", licenseData.Email, "pin_hash_prefix", pin.PinHash[:8]+"...")

	// Send PIN via email
	logger.Info("Sending PIN via email", "user_email", licenseData.Email)
	if err := sendPINEmail(ctx, licenseData.Email, randomPin); err != nil {
		logger.Error("Failed to send email", "error", err, "user_email", licenseData.Email)
		// Don't fail the request if email fails, PIN is already stored
	} else {
		logger.Info("PIN email sent successfully", "user_email", licenseData.Email)
	}

	// Return success response
	response := api.RequestPINResponse{
		Message:   fmt.Sprintf("PIN sent to %s", licenseData.Email),
		ExpiresAt: expiresAt.Format(time.RFC3339),
		PinID:     fmt.Sprintf("hash_%s", pin.PinHash[:8]), // Use hash prefix for tracking
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

func generatePIN() (string, error) {
	// Generate a random 6-digit number
	max := big.NewInt(1000000)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

func hashLicense(license string) string {
	hash := sha256.Sum256([]byte(license))
	return fmt.Sprintf("sha256:%x", hash)
}

func sendPINEmail(ctx context.Context, email, pin string) error {
	logger.Debug("Preparing PIN email", "recipient", email)
	subject := "Your BreachLine Authentication PIN"
	body := fmt.Sprintf(`Hello,

Your authentication PIN for BreachLine is: %s

This PIN will expire in %d hours.

If you did not request this PIN, please ignore this email.

Best regards,
The BreachLine Team`, pin, pinTTLHours)

	input := &ses.SendEmailInput{
		Source: &sesFromEmail,
		Destination: &sestypes.Destination{
			ToAddresses: []string{email},
		},
		Message: &sestypes.Message{
			Subject: &sestypes.Content{
				Data: &subject,
			},
			Body: &sestypes.Body{
				Text: &sestypes.Content{
					Data: &body,
				},
			},
		},
		ConfigurationSetName: &sesConfigSet,
	}

	_, err := sesClient.SendEmail(ctx, input)
	return err
}

// checkPINCooldown checks if there are any valid PINs created within the cooldown window
// This prevents creating a new PIN while a valid one exists (5 minute cooldown)
func checkPINCooldown(ctx context.Context, email string) (bool, error) {
	// Get all valid PINs for this email
	allPins, err := api.GetValidPINs(ctx, email)
	if err != nil {
		// If no PINs found, that's fine - not on cooldown
		if err.Error() == "no PINs found for email" || err.Error() == "no valid PINs found" {
			return false, nil
		}
		return false, err
	}

	// Check if any valid PIN was created within the cooldown window
	currentTime := time.Now()
	cooldownWindow := time.Duration(pinRateLimitSec) * time.Second

	for _, pin := range allPins {
		// Parse the created_at timestamp
		createdAt, err := time.Parse(time.RFC3339, pin.CreatedAt)
		if err != nil {
			logger.Warn("Failed to parse PIN created_at timestamp", "created_at", pin.CreatedAt, "error", err)
			continue
		}

		// Check if this PIN was created within the cooldown window
		if currentTime.Sub(createdAt) < cooldownWindow {
			logger.Debug("Found recent valid PIN within cooldown", "user_email", email, "created_at", pin.CreatedAt, "time_since_creation", currentTime.Sub(createdAt))
			return true, nil
		}
	}

	logger.Debug("No recent valid PINs found within cooldown", "user_email", email, "cooldown_seconds", pinRateLimitSec)
	return false, nil
}

func main() {
	lambda.Start(handler)
}
