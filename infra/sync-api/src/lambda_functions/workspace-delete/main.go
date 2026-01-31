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
	workspacesTable  string
	membersTable     string
	annotationsTable string
	filesTable       string
	auditTable       string
	rateLimitsTable  string
	logger           *slog.Logger
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	workspacesTable = os.Getenv("WORKSPACES_TABLE")
	membersTable = os.Getenv("WORKSPACE_MEMBERS_TABLE")
	annotationsTable = os.Getenv("ANNOTATIONS_TABLE")
	filesTable = os.Getenv("FILES_TABLE")
	auditTable = os.Getenv("AUDIT_TABLE")

	// Initialize the shared API layer
	api.Init(api.InitParams{
		Logger:             logger,
		WorkspacesTable:    workspacesTable,
		MembersTable:       membersTable,
		AnnotationsTable:   annotationsTable,
		FilesTable:         filesTable,
		AuditTable:         auditTable,
		SubscriptionsTable: os.Getenv("SUBSCRIPTIONS_TABLE"),
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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(workspaceDeleteHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func workspaceDeleteHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Delete workspace request received", "user_email", email)

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
		return api.CreateErrorResponse(403, "access_denied", "Only the workspace owner can delete the workspace")
	}
	logger.Debug("Ownership verified", "workspace_id", workspaceID)

	// Delete all related data
	logger.Info("Deleting workspace data", "workspace_id", workspaceID)
	if err := api.DeleteWorkspaceData(ctx, workspaceID); err != nil {
		logger.Error("Failed to delete workspace data", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to delete workspace")
	}
	logger.Info("Workspace deleted successfully", "workspace_id", workspaceID)

	// Return response
	response := api.DeleteWorkspaceResponse{
		Message: "Workspace deleted successfully",
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
