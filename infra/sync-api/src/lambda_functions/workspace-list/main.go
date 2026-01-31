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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(workspaceListHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func workspaceListHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	// Get owned workspaces
	logger.Info("Fetching owned workspaces", "user_email", email)
	ownedWorkspaces, err := api.GetOwnedWorkspaces(ctx, email)
	if err != nil {
		logger.Error("Failed to get owned workspaces", "error", err, "user_email", email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to retrieve workspaces")
	}
	logger.Debug("Owned workspaces retrieved", "user_email", email, "count", len(ownedWorkspaces))

	// Get member workspaces
	logger.Info("Fetching member workspaces", "user_email", email)
	memberWorkspaces, err := api.GetMemberWorkspaces(ctx, email)
	if err != nil {
		logger.Error("Failed to get member workspaces", "error", err, "user_email", email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to retrieve workspaces")
	}
	logger.Debug("Member workspaces retrieved", "user_email", email, "count", len(memberWorkspaces))

	// Combine results
	allWorkspaces := append(ownedWorkspaces, memberWorkspaces...)
	logger.Info("Workspace list retrieved", "user_email", email, "total_count", len(allWorkspaces))

	response := api.ListWorkspacesResponse{
		Workspaces: allWorkspaces,
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
