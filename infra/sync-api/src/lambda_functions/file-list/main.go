package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	workspacesTable string
	membersTable    string
	filesTable      string
	rateLimitsTable string
	logger          *slog.Logger
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	workspacesTable = os.Getenv("WORKSPACES_TABLE")
	membersTable = os.Getenv("WORKSPACE_MEMBERS_TABLE")
	filesTable = os.Getenv("FILES_TABLE")

	// Init API layer
	api.Init(api.InitParams{
		Logger:          logger,
		WorkspacesTable: workspacesTable,
		MembersTable:    membersTable,
		FilesTable:      filesTable,
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileListHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileListHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("File list request received", "user_email", email)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Check access
	logger.Debug("Checking workspace access", "workspace_id", workspaceID, "user_email", email)
	hasAccess, err := api.CheckWorkspaceAccess(ctx, workspaceID, email)
	if err != nil {
		logger.Error("Failed to check access", "error", err, "workspace_id", workspaceID, "user_email", email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to check access")
	}

	if !hasAccess {
		logger.Error("Access denied", "workspace_id", workspaceID, "user_email", email)
		return api.CreateErrorResponse(403, "access_denied", "You do not have access to this workspace")
	}
	logger.Debug("Access granted", "workspace_id", workspaceID, "user_email", email)

	// Query files
	logger.Debug("Fetching files", "workspace_id", workspaceID)
	files, err := api.ListFiles(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get files", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to retrieve files")
	}
	logger.Info("Files retrieved successfully", "workspace_id", workspaceID, "count", len(files))

	// Debug: log the Options values from DynamoDB
	for _, file := range files {
		if file.Options.NoHeaderRow {
			logger.Info("File has NoHeaderRow=true",
				"file_hash", file.FileHash,
				"options", file.Options)
		}
	}

	// Return files
	response := api.ListFilesResponse{
		Files: files,
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

func main() {
	lambda.Start(handler)
}
