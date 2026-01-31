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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileLocationStoreHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileLocationStoreHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing file location store request",
		"method", request.HTTPMethod,
		"path", request.Path,
	)

	// Parse request body
	var req api.StoreFileLocationRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Invalid request body"}`,
		}, nil
	}

	// Validate required fields
	if err := api.ValidateInstanceID(req.InstanceID); err != nil {
		logger.Error("Invalid instance ID", "error", err, "instance_id", req.InstanceID)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Invalid instance ID"}`,
		}, nil
	}

	if err := api.ValidateFileHash(req.FileHash); err != nil {
		logger.Error("Invalid file hash", "error", err, "file_hash", req.FileHash)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Invalid file hash"}`,
		}, nil
	}

	if err := api.ValidateAbsoluteFilePath(req.FilePath); err != nil {
		logger.Error("Invalid file path", "error", err, "file_path", req.FilePath)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Invalid file path - must be absolute"}`,
		}, nil
	}

	if req.WorkspaceID == "" {
		logger.Error("Missing workspace ID")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Workspace ID is required"}`,
		}, nil
	}

	// Create file location object
	location := api.WorkspaceFileLocation{
		InstanceID:  req.InstanceID,
		FileHash:    req.FileHash,
		WorkspaceID: req.WorkspaceID,
		FilePath:    req.FilePath,
	}

	// Store the file location
	if err := api.StoreFileLocation(ctx, location); err != nil {
		logger.Error("Failed to store file location", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Failed to store file location"}`,
		}, nil
	}

	// Return success response
	response := api.FileLocationResponse{
		Message: "File location stored successfully",
	}

	responseBody, err := json.Marshal(response)
	if err != nil {
		logger.Error("Failed to marshal response", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
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
			"Access-Control-Allow-Methods": "POST, OPTIONS",
			"Access-Control-Allow-Headers": "Content-Type, Authorization",
		},
		Body: string(responseBody),
	}, nil
}

func main() {
	lambda.Start(handler)
}
