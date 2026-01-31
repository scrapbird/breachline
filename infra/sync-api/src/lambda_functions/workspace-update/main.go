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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(workspaceUpdateHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func workspaceUpdateHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Update workspace request received", "user_email", email)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Parse request body
	var req api.UpdateWorkspaceRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()
	if !validator.ValidateUpdateWorkspaceRequest(req) {
		logger.Error("Request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	if req.Name == "" {
		logger.Error("Missing workspace name")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace name is required")
	}

	logger.Info("Parsed update request", "new_name", req.Name)

	// Get workspace and verify ownership
	logger.Debug("Fetching workspace", "workspace_id", workspaceID)
	workspace, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(404, "workspace_not_found", "Workspace not found")
	}

	if workspace.OwnerEmail != email {
		logger.Error("Access denied - not owner", "workspace_id", workspaceID, "user_email", email, "owner_email", workspace.OwnerEmail)
		return api.CreateErrorResponse(403, "access_denied", "Only the workspace owner can update the workspace")
	}
	logger.Debug("Ownership verified", "workspace_id", workspaceID)

	err = api.UpdateWorkspace(ctx, workspaceID, req.Name)
	if err != nil {
		logger.Error("Failed to update workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to update workspace")
	}
	logger.Info("Workspace updated successfully", "workspace_id", workspaceID, "new_name", req.Name)

	// Return response
	response := api.UpdateWorkspaceResponse{
		WorkspaceID: workspaceID,
		Name:        req.Name,
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
