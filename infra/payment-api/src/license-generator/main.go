package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Global cache for private key
var privateKeyCache *ecdsa.PrivateKey

// Request structure
type LicenseRequest struct {
	Email string `json:"email"`
	Days  int    `json:"days"`
}

// LicenseResponse represents the license response
type LicenseResponse struct {
	License        string `json:"license"`
	Email          string `json:"email"`
	ExpirationDate string `json:"expirationDate"`
	ValidDays      int    `json:"validDays"`
}

var (
	secretsManagerClient *secretsmanager.Client
	snsClient            *sns.Client
	secretsManagerARN    = "arn:aws:secretsmanager:ap-southeast-2:*:secret:breachline-license-signing-key*"
	licenseDeliveryTopic string
	logger               *slog.Logger
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)
	
	logger.Info("License generator lambda initializing",
		"component", "license-generator",
		"stage", "init")
	
	// Get environment variables
	licenseDeliveryTopic = os.Getenv("LICENSE_DELIVERY_TOPIC")
	logger.Info("Configuration loaded",
		"license_delivery_topic", licenseDeliveryTopic,
		"stage", "init")

	// Initialize AWS clients
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error("Failed to load AWS config", "error", err, "stage", "init")
	} else {
		secretsManagerClient = secretsmanager.NewFromConfig(cfg)
		snsClient = sns.NewFromConfig(cfg)
		logger.Info("AWS clients initialized",
			"clients", []string{"secrets-manager", "sns"},
			"stage", "init")
	}
	
	logger.Info("Initialization complete", "stage", "init")
}

// Error response structure
type ErrorResponse struct {
	Error string `json:"error"`
}

// getPrivateKey loads the private key from AWS Secrets Manager with caching
func getPrivateKey(ctx context.Context) (*ecdsa.PrivateKey, error) {
	if privateKeyCache != nil {
		logger.Info("Using cached private key")
		return privateKeyCache, nil
	}

	logger.Info("Loading private key from Secrets Manager")

	secretName := "breachline-license-signing-key"

	logger.Info("Fetching secret from Secrets Manager",
		"secret_name", secretName)
	result, err := secretsManagerClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		logger.Error("Failed to retrieve secret",
			"error", err,
			"secret_name", secretName)
		return nil, fmt.Errorf("failed to get secret value: %w", err)
	}

	if result.SecretString == nil {
		logger.Error("Secret string is nil")
		return nil, fmt.Errorf("secret string is nil")
	}

	// Parse the PEM-encoded private key
	logger.Info("Parsing EC private key from PEM format")
	privateKey, err := jwt.ParseECPrivateKeyFromPEM([]byte(*result.SecretString))
	if err != nil {
		logger.Error("Failed to parse private key", "error", err)
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	privateKeyCache = privateKey
	logger.Info("Private key loaded and cached successfully")
	return privateKey, nil
}

// generateLicense creates a JWT license signed with the private key
func generateLicense(privateKey interface{}, email string, days int) (string, error) {
	logger.Info("Generating license",
		"email", email,
		"validity_days", days)

	now := time.Now().UTC()
	expiration := now.AddDate(0, 0, days)

	// Create JWT claims
	claims := jwt.MapClaims{
		"id":    uuid.New().String(),
		"email": email,
		"nbf":   now.Unix(),
		"exp":   expiration.Unix(),
		"iat":   now.Unix(),
	}

	licenseID := claims["id"].(string)
	logger.Info("License claims created",
		"license_id", licenseID,
		"email", email,
		"valid_from", now.Format(time.RFC3339),
		"valid_until", expiration.Format(time.RFC3339))

	// Create and sign the token with ES256 algorithm
	logger.Info("Signing JWT with ES256 algorithm")
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	tokenString, err := token.SignedString(privateKey)
	if err != nil {
		logger.Error("Failed to sign token",
			"error", err,
			"email", email)
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	// Base64 encode the token (as expected by the Go application)
	logger.Info("Encoding license to base64")
	licenseContent := base64.StdEncoding.EncodeToString([]byte(tokenString))

	logger.Info("License generated successfully",
		"email", email,
		"license_id", licenseID)
	return licenseContent, nil
}

// Handler is the Lambda function handler for SNS events
func Handler(ctx context.Context, snsEvent events.SNSEvent) error {
	logger.Info("Received SNS event",
		"record_count", len(snsEvent.Records))
	
	// Process each SNS record (usually just one)
	for _, record := range snsEvent.Records {
		logger.Info("Processing SNS message",
			"message_id", record.SNS.MessageID,
			"subject", record.SNS.Subject)
		
		// Parse the SNS message body
		var licenseReq LicenseRequest
		if err := json.Unmarshal([]byte(record.SNS.Message), &licenseReq); err != nil {
			logger.Error("Failed to parse SNS message",
				"error", err,
				"message", record.SNS.Message)
			return fmt.Errorf("invalid SNS message: %w", err)
		}
		
		logger.Info("License request parsed",
			"email", licenseReq.Email,
			"validity_days", licenseReq.Days)
		
		if err := processLicenseRequest(ctx, licenseReq); err != nil {
			logger.Error("Failed to process license request",
				"error", err,
				"email", licenseReq.Email)
			return err
		}
		
		logger.Info("License request processed successfully",
			"email", licenseReq.Email)
	}
	
	return nil
}

// processLicenseRequest handles the license generation logic
func processLicenseRequest(ctx context.Context, licenseReq LicenseRequest) error {
	logger.Info("Starting license generation",
		"email", licenseReq.Email)

	// Validate input
	if licenseReq.Email == "" {
		logger.Error("Email is missing from request")
		return fmt.Errorf("missing required parameter: email")
	}

	if licenseReq.Days <= 0 {
		logger.Error("Invalid days value",
			"days", licenseReq.Days)
		return fmt.Errorf("missing required parameter: days")
	}

	// Load private key
	logger.Info("Loading signing key")
	privateKey, err := getPrivateKey(ctx)
	if err != nil {
		logger.Error("Failed to load private key", "error", err)
		return fmt.Errorf("failed to load private key: %w", err)
	}

	// Generate license
	licenseContent, err := generateLicense(privateKey, licenseReq.Email, licenseReq.Days)
	if err != nil {
		logger.Error("License generation failed",
			"error", err,
			"email", licenseReq.Email)
		return fmt.Errorf("failed to generate license: %w", err)
	}

	// Calculate expiration date
	expirationDate := time.Now().UTC().AddDate(0, 0, licenseReq.Days)

	logger.Info("License generated successfully",
		"email", licenseReq.Email,
		"expiration", expirationDate.Format(time.RFC3339),
		"validity_days", licenseReq.Days)
	
	// Publish license delivery notification to SNS
	deliveryMsg := LicenseResponse{
		License:        licenseContent,
		Email:          licenseReq.Email,
		ExpirationDate: expirationDate.Format(time.RFC3339),
		ValidDays:      licenseReq.Days,
	}
	
	messageBody, err := json.Marshal(deliveryMsg)
	if err != nil {
		logger.Error("Failed to marshal delivery message",
			"error", err,
			"email", licenseReq.Email)
		return fmt.Errorf("failed to marshal delivery message: %w", err)
	}
	logger.Info("Delivery message created")
	
	// Publish to SNS topic for email delivery
	logger.Info("Publishing license delivery notification to SNS",
		"destination_email", licenseReq.Email)
	publishInput := &sns.PublishInput{
		TopicArn: &licenseDeliveryTopic,
		Message:  stringPtr(string(messageBody)),
		Subject:  stringPtr("License Delivery"),
	}
	
	publishResult, err := snsClient.Publish(ctx, publishInput)
	if err != nil {
		logger.Error("Failed to publish to SNS",
			"error", err,
			"topic_arn", licenseDeliveryTopic,
			"email", licenseReq.Email)
		return fmt.Errorf("failed to publish license delivery notification: %w", err)
	}
	
	logger.Info("License delivery notification published successfully",
		"sns_message_id", *publishResult.MessageId,
		"email", licenseReq.Email)
	
	return nil
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

func main() {
	lambda.Start(Handler)
}
