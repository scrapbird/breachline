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

	// Initialize the shared API layer
	api.Init(api.InitParams{
		Logger:          logger,
		WorkspacesTable: workspacesTable,
		MembersTable:    membersTable,
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(updateMemberHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func updateMemberHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Update member request received", "user_email", email)

	// Get workspace ID and member email from path
	workspaceID := request.PathParameters["workspace_id"]
	memberEmail := request.PathParameters["email"]
	logger.Info("Request parameters", "workspace_id", workspaceID, "email", memberEmail)

	if workspaceID == "" || memberEmail == "" {
		logger.Error("Missing required parameters", "workspace_id", workspaceID, "email", memberEmail)
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID and member email are required")
	}

	// Parse request body
	var req api.UpdateMemberRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	if req.Role == "" {
		logger.Error("Missing role")
		return api.CreateErrorResponse(400, "invalid_request", "Role is required")
	}

	logger.Info("Parsed update request", "new_role", req.Role)

	// Validate role
	if req.Role != "editor" && req.Role != "viewer" {
		logger.Error("Invalid role", "role", req.Role)
		return api.CreateErrorResponse(400, "invalid_request", "Role must be 'editor' or 'viewer'")
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
		return api.CreateErrorResponse(403, "access_denied", "Only the workspace owner can update members")
	}
	logger.Debug("Ownership verified", "workspace_id", workspaceID, "owner_email", email)

	// Check if member exists
	logger.Debug("Checking member existence", "workspace_id", workspaceID, "email", memberEmail)
	memberExists, err := api.CheckMemberExists(ctx, workspaceID, memberEmail)
	if err != nil {
		logger.Error("Failed to check member existence", "error", err, "workspace_id", workspaceID, "email", memberEmail)
		return api.CreateErrorResponse(500, "internal_error", "Failed to check member")
	}

	if !memberExists {
		logger.Error("Member not found", "workspace_id", workspaceID, "email", memberEmail)
		return api.CreateErrorResponse(404, "member_not_found", "Member not found in workspace")
	}

	// Update member role
	logger.Info("Updating member role", "workspace_id", workspaceID, "email", memberEmail, "new_role", req.Role)
	if err := api.UpdateMemberRole(ctx, workspaceID, memberEmail, req.Role); err != nil {
		logger.Error("Failed to update member", "error", err, "workspace_id", workspaceID, "email", memberEmail)
		return api.CreateErrorResponse(500, "internal_error", "Failed to update member")
	}
	logger.Info("Member updated successfully", "workspace_id", workspaceID, "email", memberEmail, "new_role", req.Role)

	// Return response
	response := api.UpdateMemberResponse{
		Message: "Member updated successfully",
		Email:   memberEmail,
		Role:    req.Role,
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
