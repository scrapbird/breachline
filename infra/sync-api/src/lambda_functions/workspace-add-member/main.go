package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"time"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	workspacesTable    string
	membersTable       string
	subscriptionsTable string
	rateLimitsTable    string
	logger             *slog.Logger
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	workspacesTable = os.Getenv("WORKSPACES_TABLE")
	membersTable = os.Getenv("WORKSPACE_MEMBERS_TABLE")
	subscriptionsTable = os.Getenv("USER_SUBSCRIPTIONS_TABLE")

	// Initialize the shared API layer
	api.Init(api.InitParams{
		Logger:             logger,
		WorkspacesTable:    workspacesTable,
		MembersTable:       membersTable,
		SubscriptionsTable: subscriptionsTable,
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(addMemberHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func addMemberHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Add member request received", "user_email", email)

	// Get user subscription from database
	logger.Debug("Fetching user subscription", "user_email", email)
	subscription, err := api.GetUserSubscription(ctx, email)
	if err != nil {
		logger.Error("Failed to get user subscription", "error", err, "user_email", email)
		return api.CreateErrorResponse(501, "internal_error", "Failed to check seat availability")
	}
	logger.Debug("User subscription retrieved", "user_email", email, "seat_count", subscription.SeatCount)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Parse request body
	var req api.AddMemberRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	if req.Email == "" {
		logger.Error("Missing member email")
		return api.CreateErrorResponse(400, "invalid_request", "Email is required")
	}

	if req.Role == "" {
		req.Role = "editor" // Default role
	}

	logger.Info("Parsed add member request", "member_email", req.Email, "role", req.Role)

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
		return api.CreateErrorResponse(403, "access_denied", "Only the workspace owner can add members")
	}

	// Check if workspace is shared
	if !workspace.IsShared {
		logger.Error("Workspace not shared", "workspace_id", workspaceID)
		return api.CreateErrorResponse(400, "workspace_not_shared", "Workspace must be converted to shared before adding members")
	}

	// Check if member already exists
	logger.Debug("Checking if member already exists", "workspace_id", workspaceID, "member_email", req.Email)
	memberExists, err := api.CheckMemberExists(ctx, workspaceID, req.Email)
	if err != nil {
		logger.Error("Failed to check member existence", "error", err, "workspace_id", workspaceID, "member_email", req.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to check member")
	}

	if memberExists {
		logger.Error("Member already exists", "workspace_id", workspaceID, "member_email", req.Email)
		return api.CreateErrorResponse(400, "member_already_exists", "Member already exists in workspace")
	}

	// Check seat availability
	logger.Debug("Checking seat availability", "workspace_id", workspaceID, "current_members", workspace.MemberCount, "seat_count", subscription.SeatCount)
	if workspace.MemberCount >= subscription.SeatCount {
		logger.Error("Insufficient seats", "workspace_id", workspaceID, "current_members", workspace.MemberCount, "seat_count", subscription.SeatCount)
		return api.CreateErrorResponse(403, "insufficient_seats", "Not enough available seats to add this member")
	}

	// Add member
	logger.Info("Adding member to workspace", "workspace_id", workspaceID, "member_email", req.Email, "role", req.Role)
	now := time.Now().Format(time.RFC3339)
	member := api.Member{
		WorkspaceID: workspaceID,
		Email:       req.Email,
		Role:        req.Role,
		AddedAt:     now,
		LastActive:  now,
	}
	if err := api.AddMember(ctx, member); err != nil {
		logger.Error("Failed to add member", "error", err, "workspace_id", workspaceID, "member_email", req.Email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to add member")
	}

	logger.Info("Member added successfully", "workspace_id", workspaceID, "member_email", req.Email, "role", req.Role)

	// Return response
	response := api.AddMemberResponse{
		Message: "Member added successfully",
		Email:   req.Email,
		Role:    req.Role,
	}

	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 201,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
