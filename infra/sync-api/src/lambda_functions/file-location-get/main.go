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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileLocationGetHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileLocationGetHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing file location get request",
		"method", request.HTTPMethod,
		"path", request.Path,
	)

	// Parse request body
	var req api.GetFileLocationRequest
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

	// Get the file location
	location, err := api.GetFileLocation(ctx, req.InstanceID, req.FileHash)
	if err != nil {
		if err.Error() == "file location not found" {
			logger.Info("File location not found",
				"instance_id", req.InstanceID,
				"file_hash", req.FileHash,
			)
			return events.APIGatewayProxyResponse{
				StatusCode: http.StatusNotFound,
				Headers: map[string]string{
					"Content-Type":                 "application/json",
					"Access-Control-Allow-Origin":  "*",
					"Access-Control-Allow-Methods": "POST, OPTIONS",
					"Access-Control-Allow-Headers": "Content-Type, Authorization",
				},
				Body: `{"error": "File location not found"}`,
			}, nil
		}

		logger.Error("Failed to get file location", "error", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST, OPTIONS",
				"Access-Control-Allow-Headers": "Content-Type, Authorization",
			},
			Body: `{"error": "Failed to get file location"}`,
		}, nil
	}

	// Return success response with file path
	response := api.FileLocationResponse{
		Message:  "File location retrieved successfully",
		FilePath: location.FilePath,
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
