package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	workspacesTable string
	filesTable      string
	membersTable    string
	auditTable      string
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
	filesTable = os.Getenv("FILES_TABLE")
	membersTable = os.Getenv("WORKSPACE_MEMBERS_TABLE")
	auditTable = os.Getenv("AUDIT_TABLE")

	// Init API layer
	api.Init(api.InitParams{
		Logger:             logger,
		WorkspacesTable:    workspacesTable,
		FilesTable:         filesTable,
		MembersTable:       membersTable,
		AuditTable:         auditTable,
		AnnotationsTable:   os.Getenv("ANNOTATIONS_TABLE"),
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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileUpdateHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileUpdateHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("File update request received", "user_email", email)

	// Get workspace ID and file hash from path
	workspaceID := request.PathParameters["workspace_id"]
	fileHash := request.PathParameters["file_hash"]

	logger.Info("Request parameters", "workspace_id", workspaceID, "file_hash", fileHash)

	if workspaceID == "" || fileHash == "" {
		logger.Error("Missing required parameters")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID and file hash are required")
	}

	// Parse request body
	var req api.UpdateFileRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()
	if !validator.ValidateUpdateFileRequest(req) {
		logger.Error("Request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	logger.Info("Parsed update request", "file_hash", req.FileHash, "description", req.Description)

	// Get workspace
	logger.Debug("Fetching workspace", "workspace_id", workspaceID)
	workspace, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(404, "workspace_not_found", "Workspace not found")
	}
	logger.Debug("Workspace retrieved", "workspace_id", workspaceID, "owner", workspace.OwnerEmail)

	// Check if file exists (use GetFileByHash since we only have the hash from path)
	logger.Debug("Checking if file exists", "workspace_id", workspaceID, "file_hash", fileHash)
	_, err = api.GetFileByHash(ctx, workspaceID, fileHash)
	if err != nil {
		logger.Error("File not found", "error", err, "workspace_id", workspaceID, "file_hash", fileHash)
		return api.CreateErrorResponse(404, "file_not_found", "File not found")
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

	// Check write permissions
	logger.Debug("Checking write permissions", "workspace_id", workspaceID, "user_email", email)
	canWrite, err := api.CheckWritePermission(ctx, workspaceID, email, workspace.OwnerEmail)
	if err != nil {
		logger.Error("Failed to check write permission", "error", err, "workspace_id", workspaceID, "user_email", email)
		return api.CreateErrorResponse(500, "internal_error", "Failed to check permissions")
	}

	if !canWrite {
		logger.Error("Write permission denied", "workspace_id", workspaceID, "user_email", email)
		return api.CreateErrorResponse(403, "access_denied", "You do not have write permission for this workspace")
	}
	logger.Debug("Write permission granted", "workspace_id", workspaceID, "user_email", email)

	// Update file directly
	logger.Info("Updating file directly", "workspace_id", workspaceID, "file_hash", fileHash, "options", req.Options, "user_email", email)
	if err := updateFileDirect(ctx, workspaceID, fileHash, email, req); err != nil {
		logger.Error("Failed to update file", "error", err, "workspace_id", workspaceID, "file_hash", fileHash)
		return api.CreateErrorResponse(500, "internal_error", "Failed to update file")
	}
	logger.Info("File updated successfully", "workspace_id", workspaceID, "file_hash", fileHash)

	// Return response
	response := api.FileResponse{
		Message:     "File updated successfully",
		WorkspaceID: workspaceID,
		FileHash:    fileHash,
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

func updateFileDirect(ctx context.Context, workspaceID, fileHash, userEmail string, req api.UpdateFileRequest) error {
	logger.Debug("Updating file directly", "workspace_id", workspaceID, "file_hash", fileHash, "options", req.Options)

	// Build file identifier from hash and options
	fileIdentifier := api.MakeFileIdentifier(fileHash, req.Options)

	// Get current file to increment version
	currentFile, err := api.GetFile(ctx, workspaceID, fileIdentifier)
	if err != nil {
		logger.Error("Failed to get current file", "error", err, "file_hash", fileHash, "file_identifier", fileIdentifier, "workspace_id", workspaceID)
		return err
	}

	// Update file directly in DynamoDB using its file_identifier
	newVersion := currentFile.Version + 1
	err = api.UpdateFile(ctx, workspaceID, fileIdentifier, req.Description, newVersion)
	if err != nil {
		logger.Error("Failed to update file",
			"error", err,
			"file_hash", fileHash,
			"file_identifier", fileIdentifier,
			"workspace_id", workspaceID,
		)
		return err
	}

	// Update workspace timestamp
	if err := api.UpdateWorkspaceTimestamp(ctx, workspaceID); err != nil {
		logger.Error("Failed to update workspace timestamp",
			"error", err,
			"workspace_id", workspaceID,
		)
		// Don't fail the entire operation for timestamp update failure
	}

	// Create audit entry
	auditDescription := fmt.Sprintf("%s updated file %s in %s", userEmail, fileHash, workspaceID)
	if err := api.CreateAuditEntry(ctx, workspaceID, userEmail, auditDescription); err != nil {
		logger.Error("Failed to create audit entry",
			"error", err,
			"workspace_id", workspaceID,
			"file_hash", fileHash,
		)
		// Don't fail the entire operation for audit failure
	}

	logger.Debug("Successfully updated file directly",
		"file_hash", fileHash,
		"workspace_id", workspaceID,
		"new_version", newVersion,
	)

	return nil
}

func main() {
	lambda.Start(handler)
}
