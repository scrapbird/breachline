package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/scrapbird/breachline/infra/sync-api/src/api"
)

var (
	logger          *slog.Logger
	rateLimitsTable string
)

func init() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Initialize API with table names from environment
	api.Init(api.InitParams{
		Logger:             logger,
		WorkspacesTable:    os.Getenv("WORKSPACES_TABLE"),
		AnnotationsTable:   os.Getenv("ANNOTATIONS_TABLE"),
		FilesTable:         os.Getenv("FILES_TABLE"),
		AuditTable:         os.Getenv("AUDIT_TABLE"),
		SubscriptionsTable: os.Getenv("SUBSCRIPTIONS_TABLE"),
		MembersTable:       os.Getenv("MEMBERS_TABLE"),
		PinsTable:          os.Getenv("PINS_TABLE"),
		FileLocationsTable: os.Getenv("FILE_LOCATIONS_TABLE"),
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileLocationsListHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileLocationsListHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing file locations list request",
		"method", request.HTTPMethod,
		"path", request.Path,
	)

	// Parse request body to get instance ID
	var reqBody api.ListFileLocationsRequest
	if err := json.Unmarshal([]byte(request.Body), &reqBody); err != nil {
		logger.Error("Failed to parse request body", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Invalid request body"}`,
		}, nil
	}

	instanceID := reqBody.InstanceID
	if instanceID == "" {
		logger.Error("Missing instance ID in request body")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Instance ID is required"}`,
		}, nil
	}

	// Validate instance ID
	if err := api.ValidateInstanceID(instanceID); err != nil {
		logger.Error("Invalid instance ID", "error", err, "instance_id", instanceID)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Invalid instance ID"}`,
		}, nil
	}

	// Get all file locations for the instance
	locations, err := api.ListFileLocationsByInstance(ctx, instanceID)
	if err != nil {
		logger.Error("Failed to list file locations", "error", err, "instance_id", instanceID)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Failed to retrieve file locations"}`,
		}, nil
	}

	// Return success response with file locations
	response := api.ListFileLocationsResponse{
		FileLocations: locations,
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		logger.Error("Failed to marshal response", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Internal server error"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, Authorization",
		},
		Body: string(responseBody),
	}, nil
}

func main() {
	lambda.Start(handler)
}
