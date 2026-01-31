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

	// Initialize the shared API layer
	api.Init(api.InitParams{
		Logger:          logger,
		WorkspacesTable: workspacesTable,
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(workspaceConvertHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func workspaceConvertHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Convert to shared request received", "user_email", email)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Get workspace and verify ownership
	logger.Debug("Fetching workspace", "workspace_id", workspaceID)
	workspace, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(404, "workspace_not_found", "Workspace not found")
	}

	if workspace.OwnerEmail != email {
		logger.Error("Access denied - not owner", "workspace_id", workspaceID, "user_email", email, "owner_email", workspace.OwnerEmail)
		return api.CreateErrorResponse(403, "access_denied", "Only the workspace owner can convert the workspace")
	}

	if workspace.IsShared {
		logger.Error("Workspace already shared", "workspace_id", workspaceID)
		return api.CreateErrorResponse(400, "already_shared", "Workspace is already shared")
	}
	logger.Debug("Ownership verified, converting to shared", "workspace_id", workspaceID)

	err = api.ConvertToShared(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to convert workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to convert workspace")
	}
	logger.Info("Workspace converted to shared successfully", "workspace_id", workspaceID)

	// Return response
	response := api.ConvertToSharedResponse{
		WorkspaceID: workspaceID,
		IsShared:    true,
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
