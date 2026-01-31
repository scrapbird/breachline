package main

import (
	"context"
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
	"github.com/stripe/stripe-go/v78"
	"github.com/stripe/stripe-go/v78/subscription"
	"github.com/stripe/stripe-go/v78/webhook"
)

var (
	licenseGenerationTopic string
	snsClient              *sns.Client
	logger                 *slog.Logger
	// Cache for Stripe API key and webhook secret retrieved from Secrets Manager
	stripeApiKeyCache        string
	stripeWebhookSecretCache string
	// CloudFront secret for validating requests
	cloudfrontSecret string
)

// LicenseRequest represents the request to the license generator
type LicenseRequest struct {
	Email string `json:"email"`
	Days  int    `json:"days"`
}

// LicenseResponse represents the response from the license generator
type LicenseResponse struct {
	License        string `json:"license"`
	Email          string `json:"email"`
	ExpirationDate string `json:"expirationDate"`
	ValidDays      int    `json:"validDays"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Received bool `json:"received"`
}

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	logger.Info("Stripe webhook lambda initializing",
		"component", "stripe-webhook",
		"stage", "init")

	// Get environment variables
	stripeWebhookSecretArn := os.Getenv("STRIPE_WEBHOOK_SECRET_ARN")
	licenseGenerationTopic = os.Getenv("LICENSE_GENERATION_TOPIC")
	stripeAPIKeySecretArn := os.Getenv("STRIPE_API_KEY_SECRET_ARN")
	cloudfrontSecret = os.Getenv("CLOUDFRONT_SECRET")

	logger.Info("Configuration loaded",
		"license_generation_topic", licenseGenerationTopic,
		"stripe_api_key_secret_arn", stripeAPIKeySecretArn,
		"stripe_webhook_secret_arn", stripeWebhookSecretArn,
		"cloudfront_secret_set", cloudfrontSecret != "",
		"stage", "init")

	// Initialize AWS config
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		logger.Error("Failed to load AWS config", "error", err, "stage", "init")
		return
	}

	// Initialize AWS SNS client
	snsClient = sns.NewFromConfig(cfg)
	logger.Info("AWS SNS client initialized", "stage", "init")

	// Initialize Secrets Manager client
	secretsClient := secretsmanager.NewFromConfig(cfg)

	// Retrieve Stripe webhook secret from Secrets Manager
	if stripeWebhookSecretArn != "" {
		logger.Info("Fetching Stripe webhook secret from Secrets Manager")
		result, err := secretsClient.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
			SecretId: &stripeWebhookSecretArn,
		})
		if err != nil {
			logger.Error("Failed to retrieve Stripe webhook secret from Secrets Manager",
				"error", err,
				"secret_arn", stripeWebhookSecretArn,
				"stage", "init")
		} else if result.SecretString != nil {
			stripeWebhookSecretCache = *result.SecretString
			logger.Info("Stripe webhook secret retrieved from Secrets Manager", "stage", "init")
		} else {
			logger.Warn("Stripe webhook secret is empty", "stage", "init")
		}
	} else {
		logger.Warn("STRIPE_WEBHOOK_SECRET_ARN not set", "stage", "init")
	}

	// Retrieve Stripe API key from Secrets Manager (cached globally)
	if stripeAPIKeySecretArn != "" {
		logger.Info("Fetching Stripe API key from Secrets Manager")
		result, err := secretsClient.GetSecretValue(context.Background(), &secretsmanager.GetSecretValueInput{
			SecretId: &stripeAPIKeySecretArn,
		})
		if err != nil {
			logger.Error("Failed to retrieve Stripe API key from Secrets Manager",
				"error", err,
				"secret_arn", stripeAPIKeySecretArn,
				"stage", "init")
		} else if result.SecretString != nil {
			// Cache the secret key in global variable
			stripeApiKeyCache = *result.SecretString
			stripe.Key = stripeApiKeyCache
			logger.Info("Stripe API key retrieved and cached from Secrets Manager", "stage", "init")
		} else {
			logger.Warn("Stripe API key secret is empty", "stage", "init")
		}
	} else {
		logger.Warn("STRIPE_API_KEY_SECRET_ARN not set", "stage", "init")
	}

	logger.Info("Initialization complete", "stage", "init")
}

// Handler is the Lambda function handler for Stripe webhook events
func Handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Validate CloudFront secret header first (blocks direct API Gateway access)
	cloudfrontSecretHeader := request.Headers["x-cloudfront-secret"]
	if cloudfrontSecretHeader == "" {
		cloudfrontSecretHeader = request.Headers["X-CloudFront-Secret"]
	}
	
	if cloudfrontSecret != "" && cloudfrontSecretHeader != cloudfrontSecret {
		logger.Warn("Request rejected: invalid or missing CloudFront secret",
			"reason", "unauthorized_direct_access",
			"has_header", cloudfrontSecretHeader != "")
		errorBody, _ := json.Marshal(ErrorResponse{Error: "Forbidden"})
		return events.APIGatewayProxyResponse{
			StatusCode: 403,
			Body:       string(errorBody),
		}, nil
	}
	
	// Get the Stripe signature from headers
	signature := request.Headers["stripe-signature"]
	if signature == "" {
		signature = request.Headers["Stripe-Signature"]
	}

	if signature == "" {
		logger.Warn("Webhook rejected: no signature",
			"reason", "missing_signature",
			"headers", request.Headers)
		errorBody, _ := json.Marshal(ErrorResponse{Error: "No signature provided"})
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       string(errorBody),
		}, nil
	}

	// Get the raw body
	body := request.Body

	// Verify webhook signature
	logger.Info("Verifying webhook signature")
	event, err := webhook.ConstructEvent([]byte(body), signature, stripeWebhookSecretCache)
	if err != nil {
		logger.Error("Webhook signature verification failed",
			"error", err,
			"reason", "invalid_signature")
		errorBody, _ := json.Marshal(ErrorResponse{Error: "Invalid signature"})
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       string(errorBody),
		}, nil
	}

	logger.Info("Webhook signature verified",
		"event_type", event.Type,
		"event_id", event.ID)

	// Handle checkout.session.completed event
	if event.Type == "checkout.session.completed" {
		logger.Info("Processing checkout session completed event",
			"event_type", event.Type)
		var session stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &session); err != nil {
			logger.Error("Failed to parse checkout session",
				"error", err)
			errorBody, _ := json.Marshal(ErrorResponse{Error: "Failed to parse session"})
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       string(errorBody),
			}, nil
		}
		logger.Info("Checkout session parsed",
			"session_id", session.ID)

		// Process the successful checkout
		logger.Info("Initiating checkout session processing")
		success, errMsg := processCheckoutSession(&session)
		if !success {
			logger.Error("Failed to process checkout session",
				"error_message", errMsg,
				"session_id", session.ID)
			errorBody, _ := json.Marshal(ErrorResponse{Error: "Processing failed"})
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       string(errorBody),
			}, nil
		}
		logger.Info("Checkout session processed successfully",
			"session_id", session.ID)

		successBody, _ := json.Marshal(SuccessResponse{Received: true})
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Body:       string(successBody),
		}, nil
	}

	// Return success for other event types (we don't process them)
	logger.Info("Event acknowledged but not processed",
		"event_type", event.Type,
		"reason", "not_checkout_session_completed")
	successBody, _ := json.Marshal(SuccessResponse{Received: true})
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(successBody),
	}, nil
}

// processCheckoutSession processes a successful Stripe checkout session and generates a license
func processCheckoutSession(session *stripe.CheckoutSession) (bool, string) {
	// Extract customer information
	customerEmail := ""
	if session.CustomerDetails != nil {
		customerEmail = session.CustomerDetails.Email
	}

	if customerEmail == "" {
		logger.Error("No customer email in checkout session",
			"session_id", session.ID)
		return false, "No customer email"
	}
	logger.Info("Customer email extracted",
		"email", customerEmail,
		"session_id", session.ID)

	// Get subscription details
	subscriptionID := ""
	if session.Subscription != nil {
		subscriptionID = session.Subscription.ID
	}

	// Calculate license duration based on subscription interval
	licenseDays := 365 // Default to yearly

	if subscriptionID != "" {
		logger.Info("Fetching subscription details",
			"subscription_id", subscriptionID)
		sub, err := subscription.Get(subscriptionID, nil)
		if err != nil {
			logger.Warn("Failed to retrieve subscription, using default",
				"error", err,
				"subscription_id", subscriptionID,
				"default_days", 365)
		} else {
			// Get the plan from subscription items
			if sub.Items != nil && len(sub.Items.Data) > 0 {
				price := sub.Items.Data[0].Price
				if price != nil && price.Recurring != nil {
					interval := price.Recurring.Interval
					if interval == stripe.PriceRecurringIntervalMonth {
						// Calculate days until next month from now
						now := time.Now().UTC()
						nextMonth := now.AddDate(0, 1, 0)
						licenseDays = int(nextMonth.Sub(now).Hours() / 24)
						logger.Info("Monthly subscription detected",
							"interval", "monthly",
							"license_days", licenseDays)
					} else if interval == stripe.PriceRecurringIntervalYear {
						licenseDays = 365
						logger.Info("Yearly subscription detected",
							"interval", "yearly",
							"license_days", 365)
					}
				}
			}
		}
	}

	logger.Info("License generation request",
		"email", customerEmail,
		"validity_days", licenseDays,
		"session_id", session.ID)

	// Publish license generation request to SNS
	licenseReq := LicenseRequest{
		Email: customerEmail,
		Days:  licenseDays,
	}

	messageBody, err := json.Marshal(licenseReq)
	if err != nil {
		logger.Error("Failed to marshal license request",
			"error", err,
			"email", customerEmail)
		return false, fmt.Sprintf("Failed to marshal request: %v", err)
	}
	logger.Info("License request payload created")

	// Publish to SNS topic
	publishInput := &sns.PublishInput{
		TopicArn: &licenseGenerationTopic,
		Message:  stringPtr(string(messageBody)),
		Subject:  stringPtr("License Generation Request"),
	}

	logger.Info("Publishing license generation request to SNS")
	publishResult, err := snsClient.Publish(context.Background(), publishInput)
	if err != nil {
		logger.Error("Failed to publish to SNS",
			"error", err,
			"topic_arn", licenseGenerationTopic,
			"email", customerEmail)
		return false, fmt.Sprintf("Failed to publish to SNS: %v", err)
	}

	logger.Info("License generation request published successfully",
		"sns_message_id", *publishResult.MessageId,
		"email", customerEmail,
		"validity_days", licenseDays)

	return true, ""
}

// stringPtr returns a pointer to the given string
func stringPtr(s string) *string {
	return &s
}

func main() {
	lambda.Start(Handler)
}
