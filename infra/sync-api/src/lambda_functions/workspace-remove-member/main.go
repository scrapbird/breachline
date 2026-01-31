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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(removeMemberHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func removeMemberHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	email := request.RequestContext.Authorizer["email"].(string)
	workspaceID := request.PathParameters["workspace_id"]
	memberEmail := request.PathParameters["email"]

	logger.Info("Remove member request received", "user_email", email, "workspace_id", workspaceID, "email", memberEmail)

	if workspaceID == "" || memberEmail == "" {
		logger.Error("Missing required parameters", "workspace_id", workspaceID, "email", memberEmail)
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID and member email are required")
	}

	logger.Debug("Fetching workspace", "workspace_id", workspaceID)
	workspace, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(404, "workspace_not_found", "Workspace not found")
	}

	if workspace.OwnerEmail != email {
		logger.Error("Access denied - not owner", "workspace_id", workspaceID, "user_email", email, "owner_email", workspace.OwnerEmail)
		return api.CreateErrorResponse(403, "access_denied", "Only the workspace owner can remove members")
	}
	logger.Debug("Ownership verified", "workspace_id", workspaceID)

	logger.Debug("Checking member existence", "workspace_id", workspaceID, "email", memberEmail)
	memberExists, err := api.CheckMemberExists(ctx, workspaceID, memberEmail)
	if err != nil || !memberExists {
		logger.Error("Member not found", "workspace_id", workspaceID, "email", memberEmail, "error", err)
		return api.CreateErrorResponse(404, "member_not_found", "Member not found")
	}

	logger.Info("Removing member", "workspace_id", workspaceID, "email", memberEmail)
	if err := api.RemoveMember(ctx, workspaceID, memberEmail); err != nil {
		logger.Error("Failed to remove member", "error", err, "workspace_id", workspaceID, "email", memberEmail)
		return api.CreateErrorResponse(500, "internal_error", "Failed to remove member")
	}

	logger.Info("Member removed successfully", "workspace_id", workspaceID, "email", memberEmail)

	response := api.RemoveMemberResponse{Message: "Member removed successfully"}
	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
