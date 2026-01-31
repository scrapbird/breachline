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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileGetHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileGetHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("File get request received", "user_email", email)

	// Get workspace ID and file hash from path
	workspaceID := request.PathParameters["workspace_id"]
	fileHash := request.PathParameters["file_hash"]

	logger.Info("Request parameters", "workspace_id", workspaceID, "file_hash", fileHash)

	if workspaceID == "" || fileHash == "" {
		logger.Error("Missing required parameters", "workspace_id", workspaceID, "file_hash", fileHash)
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID and file hash are required")
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

	// Get file
	logger.Debug("Fetching file", "workspace_id", workspaceID, "file_hash", fileHash)
	file, err := api.GetFile(ctx, workspaceID, fileHash)
	if err != nil {
		logger.Error("Failed to get file", "error", err, "workspace_id", workspaceID, "file_hash", fileHash)
		return api.CreateErrorResponse(404, "file_not_found", "File not found")
	}
	logger.Info("File retrieved successfully", "workspace_id", workspaceID, "file_hash", fileHash)

	body, _ := json.Marshal(file)
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
