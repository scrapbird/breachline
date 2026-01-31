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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(fileDeleteHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func fileDeleteHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("File delete request received", "user_email", email)

	// Get workspace ID and file hash from path
	workspaceID := request.PathParameters["workspace_id"]
	fileHash := request.PathParameters["file_hash"]

	// Parse request body for FileOptions
	var req api.DeleteFileRequest
	if err := json.Unmarshal([]byte(request.Body), &req); err != nil {
		logger.Error("Failed to parse request body", "error", err, "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
	}

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()
	if !validator.ValidateDeleteFileRequest(req) {
		logger.Error("Request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	// Validate that the file hash in the path matches the request body
	if req.FileHash != fileHash {
		logger.Error("File hash mismatch", "path_hash", fileHash, "request_hash", req.FileHash)
		return api.CreateErrorResponse(400, "invalid_request", "File hash in path does not match request body")
	}

	// Build file identifier from hash and options
	fileIdentifier := api.MakeFileIdentifier(req.FileHash, req.Options)

	logger.Info("Request parameters",
		"workspace_id", workspaceID,
		"file_hash", req.FileHash,
		"options", req.Options,
		"file_identifier", fileIdentifier)

	if workspaceID == "" || req.FileHash == "" {
		logger.Error("Missing required parameters")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID and file hash are required")
	}

	// Get workspace
	logger.Debug("Fetching workspace", "workspace_id", workspaceID)
	workspace, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get workspace", "error", err, "workspace_id", workspaceID)
		return api.CreateErrorResponse(404, "workspace_not_found", "Workspace not found")
	}
	logger.Debug("Workspace retrieved", "workspace_id", workspaceID, "owner", workspace.OwnerEmail)

	// Note: We skip the file existence check here because it uses a GSI that may have replication lag.
	// The DeleteFileByHash function will list all files and filter by hash, handling the case where
	// the file doesn't exist gracefully (it will simply delete 0 files).

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

	// Delete file directly - passing full file identifier to delete only this specific variant
	logger.Info("Deleting file directly", "workspace_id", workspaceID, "file_identifier", fileIdentifier, "user_email", email)
	if err := deleteFileDirect(ctx, workspaceID, req.FileHash, fileIdentifier, req.Options, email); err != nil {
		logger.Error("Failed to delete file", "error", err, "workspace_id", workspaceID, "file_identifier", fileIdentifier)
		return api.CreateErrorResponse(500, "internal_error", "Failed to delete file")
	}
	logger.Info("File deleted successfully", "workspace_id", workspaceID, "file_identifier", fileIdentifier)

	// Return response
	response := api.FileResponse{
		Message:     "File deleted successfully",
		WorkspaceID: workspaceID,
		FileHash:    req.FileHash,
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

func deleteFileDirect(ctx context.Context, workspaceID, fileHash, fileIdentifier string, opts api.FileOptions, userEmail string) error {
	logger.Debug("Deleting specific file variant",
		"workspace_id", workspaceID,
		"file_identifier", fileIdentifier,
		"file_hash", fileHash,
		"options", opts,
	)

	// Delete only this specific file record (not all files with the same hash)
	err := api.DeleteFile(ctx, workspaceID, fileIdentifier)
	if err != nil {
		logger.Error("Failed to delete file",
			"error", err,
			"file_identifier", fileIdentifier,
			"workspace_id", workspaceID,
		)
		return err
	}

	// Delete annotations only for this specific file variant
	annotDeleteCount, err := api.DeleteAnnotationsByFileVariant(ctx, workspaceID, fileHash, opts.JPath, opts.NoHeaderRow, opts.IngestTimezoneOverride)
	if err != nil {
		logger.Error("Failed to delete annotations for file variant",
			"error", err,
			"file_identifier", fileIdentifier,
			"workspace_id", workspaceID,
		)
		// Don't fail the entire operation for annotation deletion failure
	} else {
		logger.Info("Deleted annotations for file variant",
			"file_identifier", fileIdentifier,
			"workspace_id", workspaceID,
			"annotation_count", annotDeleteCount,
		)
	}

	// Only delete file locations if no other files with this hash remain in the workspace
	// (same file can be loaded with different options like noHeaderRow or timezone)
	remainingFiles, err := api.ListFiles(ctx, workspaceID)
	locationDeleteCount := 0
	if err != nil {
		logger.Error("Failed to check remaining files for file location cleanup",
			"error", err,
			"file_hash", fileHash,
			"workspace_id", workspaceID,
		)
	} else {
		// Check if any files with this hash still exist
		hashStillExists := false
		for _, file := range remainingFiles {
			if file.FileHash == fileHash {
				hashStillExists = true
				break
			}
		}

		if !hashStillExists {
			// No other files with this hash exist, safe to delete file locations
			locationDeleteCount, err = api.DeleteFileLocationsByFileHash(ctx, workspaceID, fileHash)
			if err != nil {
				logger.Error("Failed to delete file locations for file",
					"error", err,
					"file_hash", fileHash,
					"workspace_id", workspaceID,
				)
			} else {
				logger.Info("Deleted file locations for file",
					"file_hash", fileHash,
					"workspace_id", workspaceID,
					"location_count", locationDeleteCount,
				)
			}
		} else {
			logger.Info("Skipping file location deletion - other files with same hash still exist",
				"file_hash", fileHash,
				"workspace_id", workspaceID,
			)
		}
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
	auditDescription := fmt.Sprintf("%s deleted file %s (and %d annotations, %d locations) from %s", userEmail, fileIdentifier, annotDeleteCount, locationDeleteCount, workspaceID)
	if err := api.CreateAuditEntry(ctx, workspaceID, userEmail, auditDescription); err != nil {
		logger.Error("Failed to create audit entry",
			"error", err,
			"workspace_id", workspaceID,
			"file_hash", fileHash,
		)
		// Don't fail the entire operation for audit failure
	}

	logger.Debug("Successfully deleted file directly",
		"file_hash", fileHash,
		"workspace_id", workspaceID,
	)

	return nil
}

func main() {
	lambda.Start(handler)
}
