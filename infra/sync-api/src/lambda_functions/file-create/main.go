package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	workspacesTable    string
	filesTable         string
	membersTable       string
	fileLocationsTable string
	auditTable         string
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
	filesTable = os.Getenv("FILES_TABLE")
	membersTable = os.Getenv("WORKSPACE_MEMBERS_TABLE")
	fileLocationsTable = os.Getenv("FILE_LOCATIONS_TABLE")
	auditTable = os.Getenv("AUDIT_TABLE")

	// Init API layer
	api.Init(api.InitParams{
		Logger:             logger,
		WorkspacesTable:    workspacesTable,
		FilesTable:         filesTable,
		MembersTable:       membersTable,
		FileLocationsTable: fileLocationsTable,
		AuditTable:         auditTable,
		AnnotationsTable:   os.Getenv("ANNOTATIONS_TABLE"),
		SubscriptionsTable: os.Getenv("SUBSCRIPTIONS_TABLE"),
		PinsTable:          os.Getenv("PINS_TABLE"),
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileCreateHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileCreateHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("File create request received", "user_email", email)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Parse request body
	var req api.CreateFileRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	// Set workspace_id from path parameter before validation
	req.WorkspaceID = workspaceID

	// Validate required fields first
	if req.FileHash == "" {
		logger.Error("Missing required fields", "has_file_hash", req.FileHash != "")
		return api.CreateErrorResponse(400, "invalid_request", "file_hash is required")
	}

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()
	if !validator.ValidateCreateFileRequest(req) {
		logger.Error("Request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	logger.Info("Parsed create request", "file_hash", req.FileHash, "options", req.Options)

	// Get workspace
	logger.Debug("Fetching workspace", "workspace_id", workspaceID)
	workspace, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(404, "workspace_not_found", "Workspace not found")
	}
	logger.Debug("Workspace retrieved", "workspace_id", workspaceID, "owner", workspace.OwnerEmail)

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

	// Create file directly
	logger.Info("Creating file directly", "file_hash", req.FileHash, "workspace_id", workspaceID)
	if err := createFileDirect(ctx, workspaceID, email, req); err != nil {
		logger.Error("Failed to create file", "error", err, "file_hash", req.FileHash)
		return api.CreateErrorResponse(500, "internal_error", "Failed to create file")
	}

	logger.Info("File created successfully", "file_hash", req.FileHash, "workspace_id", workspaceID)

	// Store file location if file path and instance ID are provided
	if req.FilePath != "" && req.InstanceID != "" {
		logger.Info("Storing file location", "workspace_id", workspaceID, "file_hash", req.FileHash, "file_path", req.FilePath, "instance_id", req.InstanceID)
		if err := storeFileLocation(ctx, workspaceID, req.FileHash, req.FilePath, req.InstanceID); err != nil {
			// Log error but don't fail the entire request - file creation is more important
			logger.Error("Failed to store file location", "error", err, "workspace_id", workspaceID, "file_hash", req.FileHash, "file_path", req.FilePath, "instance_id", req.InstanceID)
		} else {
			logger.Info("File location stored successfully", "workspace_id", workspaceID, "file_hash", req.FileHash, "instance_id", req.InstanceID)
		}
	}

	// Return success response
	response := api.FileResponse{
		Message:     "File created successfully",
		FileHash:    req.FileHash,
		WorkspaceID: workspaceID,
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

func createFileDirect(ctx context.Context, workspaceID, userEmail string, req api.CreateFileRequest) error {
	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339)

	logger.Debug("Creating file directly",
		"workspace_id", workspaceID,
		"file_hash", req.FileHash,
		"options", req.Options)

	// Create file directly in DynamoDB
	file := api.WorkspaceFile{
		WorkspaceID: workspaceID,
		FileHash:    req.FileHash,
		Options:     req.Options,
		Description: req.Description,
		Version:     1,
		CreatedBy:   userEmail,
		CreatedAt:   createdAt,
		UpdatedAt:   createdAt,
	}

	// Debug: log the options being stored
	logger.Info("Storing file with options",
		"file_hash", req.FileHash,
		"options", file.Options)

	err := api.CreateFile(ctx, file)
	if err != nil {
		logger.Error("Failed to create file",
			"error", err,
			"file_hash", req.FileHash,
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
	auditDescription := fmt.Sprintf("%s added file %s to %s", userEmail, req.FileHash, workspaceID)
	if err := api.CreateAuditEntry(ctx, workspaceID, userEmail, auditDescription); err != nil {
		logger.Error("Failed to create audit entry",
			"error", err,
			"workspace_id", workspaceID,
			"file_hash", req.FileHash,
		)
		// Don't fail the entire operation for audit failure
	}

	logger.Debug("Successfully created file directly",
		"file_hash", req.FileHash,
		"workspace_id", workspaceID,
	)

	return nil
}

func storeFileLocation(ctx context.Context, workspaceID, fileHash, filePath, instanceID string) error {
	logger.Debug("Storing file location",
		"instance_id", instanceID,
		"workspace_id", workspaceID,
		"file_hash", fileHash,
		"file_path", filePath,
	)

	location := api.WorkspaceFileLocation{
		InstanceID:  instanceID,
		FileHash:    fileHash,
		WorkspaceID: workspaceID,
		FilePath:    filePath,
	}

	return api.StoreFileLocation(ctx, location)
}

func main() {
	lambda.Start(handler)
}
