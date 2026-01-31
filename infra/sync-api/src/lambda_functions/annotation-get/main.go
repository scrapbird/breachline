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

	// Init API layer
	api.Init(api.InitParams{
		Logger:           logger,
		WorkspacesTable:  workspacesTable,
		MembersTable:     membersTable,
		AnnotationsTable: annotationsTable,
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(annotationGetHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func annotationGetHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Annotation get request received", "user_email", email)

	// Get workspace ID and annotation ID from path
	workspaceID := request.PathParameters["workspace_id"]
	annotationID := request.PathParameters["annotation_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID, "annotation_id", annotationID)

	if workspaceID == "" || annotationID == "" {
		logger.Error("Missing required parameters", "workspace_id", workspaceID, "annotation_id", annotationID)
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID and Annotation ID are required")
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

	// Get annotation
	logger.Debug("Fetching annotation", "workspace_id", workspaceID, "annotation_id", annotationID)
	annotation, err := api.GetAnnotation(ctx, workspaceID, annotationID)
	if err != nil {
		logger.Error("Failed to get annotation", "error", err, "workspace_id", workspaceID, "annotation_id", annotationID)
		return api.CreateErrorResponse(404, "annotation_not_found", "Annotation not found")
	}
	logger.Info("Annotation retrieved successfully", "workspace_id", workspaceID, "annotation_id", annotationID)

	body, _ := json.Marshal(annotation)
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
