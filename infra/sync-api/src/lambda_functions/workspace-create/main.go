package main

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var (
	ddbClient          *dynamodb.Client
	workspacesTable    string
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

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	ddbClient = dynamodb.NewFromConfig(cfg)
	workspacesTable = os.Getenv("WORKSPACES_TABLE")
	subscriptionsTable = os.Getenv("USER_SUBSCRIPTIONS_TABLE")

	// Initialize the shared API layer
	api.Init(api.InitParams{
		Logger:             logger,
		WorkspacesTable:    workspacesTable,
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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(workspaceCreateHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func workspaceCreateHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Create workspace request received", "user_email", email)

	// Get user subscription from database
	logger.Debug("Fetching user subscription", "user_email", email)
	subscription, err := api.GetUserSubscription(ctx, email)
	if err != nil {
		logger.Error("Failed to get user subscription", "error", err, "user_email", email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to check workspace limit")
	}
	logger.Debug("User subscription retrieved", "user_email", email, "workspace_limit", subscription.WorkspaceLimit)

	// Parse request body
	var req api.CreateWorkspaceRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()
	if !validator.ValidateCreateWorkspaceRequest(req) {
		logger.Error("Request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	if req.Name == "" {
		logger.Error("Missing workspace name")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace name is required")
	}

	logger.Info("Parsed create request", "workspace_name", req.Name)

	// Check current workspace count
	logger.Debug("Checking workspace count", "user_email", email)
	currentCount, err := api.GetWorkspaceCount(ctx, email)
	if err != nil {
		logger.Error("Failed to get workspace count", "error", err, "user_email", email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to check workspace limit")
	}

	logger.Debug("Workspace count retrieved", "user_email", email, "current_count", currentCount, "limit", subscription.WorkspaceLimit)

	if currentCount >= subscription.WorkspaceLimit {
		logger.Error("Workspace limit exceeded", "user_email", email, "current_count", currentCount, "limit", subscription.WorkspaceLimit)
		return api.CreateErrorResponse(403, "workspace_limit_exceeded", "Workspace limit exceeded")
	}

	// Generate hash key for the workspace
	hashKey, err := api.GenerateHashKey()
	if err != nil {
		logger.Error("Failed to generate hash key", "error", err)
		return api.CreateErrorResponse(500, "internal_error", "Failed to generate workspace hash key")
	}
	logger.Info("Generated hash key for workspace", "hash_key_length", len(hashKey), "workspace_name", req.Name)

	// Create workspace
	now := time.Now().Format(time.RFC3339)
	workspace := api.Workspace{
		WorkspaceID: api.GenerateWorkspaceID(),
		HashKey:     hashKey,
		Name:        req.Name,
		OwnerEmail:  email,
		IsShared:    false,
		MemberCount: 0,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	logger.Info("About to create workspace in DynamoDB", "workspace_id", workspace.WorkspaceID, "hash_key_length", len(workspace.HashKey))
	err = api.CreateWorkspace(ctx, workspace)
	if err != nil {
		logger.Error("Failed to create workspace", "error", err, "workspace_id", workspace.WorkspaceID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to create workspace")
	}
	logger.Info("Workspace created successfully", "workspace_id", workspace.WorkspaceID, "workspace_name", workspace.Name, "owner_email", email, "hash_key_stored", workspace.HashKey != "")

	// Return response
	response := api.CreateWorkspaceResponse{
		WorkspaceID: workspace.WorkspaceID,
		HashKey:     workspace.HashKey,
		Name:        workspace.Name,
		OwnerEmail:  workspace.OwnerEmail,
		IsShared:    workspace.IsShared,
		Version:     workspace.Version,
		CreatedAt:   workspace.CreatedAt,
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
