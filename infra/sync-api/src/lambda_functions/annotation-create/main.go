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
	workspacesTable  string
	annotationsTable string
	membersTable     string
	auditTable       string
	rateLimitsTable  string
	logger           *slog.Logger
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)

	workspacesTable = os.Getenv("WORKSPACES_TABLE")
	annotationsTable = os.Getenv("ANNOTATIONS_TABLE")
	membersTable = os.Getenv("WORKSPACE_MEMBERS_TABLE")
	auditTable = os.Getenv("AUDIT_TABLE")

	// Init API layer
	api.Init(api.InitParams{
		Logger:             logger,
		WorkspacesTable:    workspacesTable,
		AnnotationsTable:   annotationsTable,
		MembersTable:       membersTable,
		AuditTable:         auditTable,
		FilesTable:         os.Getenv("FILES_TABLE"),
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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(annotationCreateHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func annotationCreateHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Annotation create request received", "user_email", email)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Detect request type by trying to parse as batch first, then single
	var isBatch bool
	var batchReq api.BatchCreateAnnotationRequest
	var singleReq api.CreateAnnotationRequest

	// Try parsing as batch request first
	if err := json.Unmarshal([]byte(request.Body), &batchReq); err == nil && len(batchReq.AnnotationRows) > 0 {
		isBatch = true
		logger.Info("Detected batch annotation request", "annotation_count", len(batchReq.AnnotationRows))
	} else {
		// Try parsing as single request
		if err := json.Unmarshal([]byte(request.Body), &singleReq); err != nil {
			logger.Error("Failed to parse request body as single or batch", "error", err, "body_length", len(request.Body))
			return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
		}
		isBatch = false
		logger.Info("Detected single annotation request")
	}

	// Set workspace_id from path parameter before validation
	singleReq.WorkspaceID = workspaceID
	batchReq.WorkspaceID = workspaceID

	// Common validation and permission checks
	_, err := validateWorkspaceAndPermissions(ctx, workspaceID, email)
	if err != nil {
		return *err, nil
	}

	if isBatch {
		return handleBatchRequest(ctx, workspaceID, email, batchReq)
	} else {
		return handleSingleRequest(ctx, workspaceID, email, singleReq)
	}
}

func validateWorkspaceAndPermissions(ctx context.Context, workspaceID, email string) (*api.Workspace, *events.APIGatewayProxyResponse) {
	// Get workspace
	logger.Debug("Fetching workspace", "workspace_id", workspaceID)
	workspace, err := api.GetWorkspace(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to get workspace", "error", err, "workspace_id", workspaceID)
		response, _ := api.CreateErrorResponse(404, "workspace_not_found", "Workspace not found")
		return nil, &response
	}
	logger.Debug("Workspace retrieved", "workspace_id", workspaceID, "owner", workspace.OwnerEmail)

	// Check access
	logger.Debug("Checking workspace access", "workspace_id", workspaceID, "user_email", email)
	hasAccess, err := api.CheckWorkspaceAccess(ctx, workspaceID, email)
	if err != nil {
		logger.Error("Failed to check access", "error", err, "workspace_id", workspaceID, "user_email", email)
		response, _ := api.CreateErrorResponse(500, "internal_error", "Failed to check access")
		return nil, &response
	}

	if !hasAccess {
		logger.Error("Access denied", "workspace_id", workspaceID, "user_email", email)
		response, _ := api.CreateErrorResponse(403, "access_denied", "You do not have access to this workspace")
		return nil, &response
	}
	logger.Debug("Access granted", "workspace_id", workspaceID, "user_email", email)

	// Check write permissions
	logger.Debug("Checking write permissions", "workspace_id", workspaceID, "user_email", email)
	canWrite, err := api.CheckWritePermission(ctx, workspaceID, email, workspace.OwnerEmail)
	if err != nil {
		logger.Error("Failed to check write permission", "error", err, "workspace_id", workspaceID, "user_email", email)
		response, _ := api.CreateErrorResponse(500, "internal_error", "Failed to check permissions")
		return nil, &response
	}

	if !canWrite {
		logger.Error("Write permission denied", "workspace_id", workspaceID, "user_email", email)
		response, _ := api.CreateErrorResponse(403, "access_denied", "You do not have write permission for this workspace")
		return nil, &response
	}
	logger.Debug("Write permission granted", "workspace_id", workspaceID, "user_email", email)

	return workspace, nil
}

func handleSingleRequest(ctx context.Context, workspaceID, email string, req api.CreateAnnotationRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing single annotation request", "file_hash", req.FileHash, "color", req.Color, "note_length", len(req.Note), "row_index", req.RowIndex)

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()
	if !validator.ValidateCreateAnnotationRequest(req) {
		logger.Error("Request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	// Validate required fields
	if req.FileHash == "" || req.Color == "" {
		logger.Error("Missing required fields",
			"has_file_hash", req.FileHash != "",
			"has_note", req.Note != "",
			"has_color", req.Color != "")
		return api.CreateErrorResponse(400, "invalid_request", "file_hash and color are required")
	}
	// RowIndex validation: must be non-negative (0 is valid for first row)
	if req.RowIndex < 0 {
		logger.Error("Invalid row index", "row_index", req.RowIndex)
		return api.CreateErrorResponse(400, "invalid_request", "row_index cannot be negative")
	}

	// Generate annotation ID
	annotationID := api.GenerateAnnotationID()
	logger.Info("Generated annotation ID", "annotation_id", annotationID, "workspace_id", workspaceID)

	// Create annotation directly
	logger.Info("Creating annotation directly", "workspace_id", workspaceID, "annotation_id", annotationID, "user_email", email)
	if err := createAnnotationDirect(ctx, workspaceID, email, annotationID, req); err != nil {
		logger.Error("Failed to create annotation", "error", err, "workspace_id", workspaceID, "annotation_id", annotationID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to create annotation")
	}
	logger.Info("Single annotation created successfully", "workspace_id", workspaceID, "annotation_id", annotationID)

	// Return response
	response := api.AnnotationResponse{
		Message:      "Annotation created successfully",
		AnnotationID: annotationID,
		WorkspaceID:  workspaceID,
	}

	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 202,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func handleBatchRequest(ctx context.Context, workspaceID, email string, req api.BatchCreateAnnotationRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing batch annotation request", "file_hash", req.FileHash, "color", req.Color, "note_length", len(req.Note), "annotation_count", len(req.AnnotationRows))

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()

	// Validate batch request fields
	if !validator.ValidateID(req.WorkspaceID, "workspace_id") {
		logger.Error("Batch request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}
	if !validator.ValidateHash(req.FileHash, "file_hash") {
		logger.Error("Batch request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}
	if !validator.ValidateAnnotationNote(req.Note) {
		logger.Error("Batch request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}
	if !validator.ValidateAnnotationColor(string(req.Color)) {
		logger.Error("Batch request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}
	if !validator.ValidateJPath(req.Options.JPath) {
		logger.Error("Batch request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	// Validate required fields
	if req.FileHash == "" || req.AnnotationRows == nil || len(req.AnnotationRows) == 0 || req.Color == "" {
		logger.Error("Missing required fields for batch request",
			"has_file_hash", req.FileHash != "",
			"has_annotation_rows", req.AnnotationRows != nil,
			"annotation_rows_count", len(req.AnnotationRows),
			"has_note", req.Note != "",
			"has_color", req.Color != "")
		return api.CreateErrorResponse(400, "invalid_request", "file_hash, annotation_rows (non-empty), note, and color are required")
	}

	// Validate batch size limit (max 256)
	if len(req.AnnotationRows) > 256 {
		logger.Error("Batch size exceeds limit", "annotation_count", len(req.AnnotationRows), "max_allowed", 256)
		return api.CreateErrorResponse(400, "batch_size_exceeded", "Maximum batch size is 256 annotations")
	}

	// Validate each annotation row
	for i, row := range req.AnnotationRows {
		if row.RowIndex < 0 {
			logger.Error("Invalid annotation row", "batch_index", i, "row_index", row.RowIndex)
			return api.CreateErrorResponse(400, "invalid_request", fmt.Sprintf("annotation_rows[%d] must have non-negative row_index", i))
		}
	}

	// Generate annotation IDs for all annotations in the batch
	annotationIDs := make([]string, len(req.AnnotationRows))
	for i := range req.AnnotationRows {
		annotationIDs[i] = api.GenerateAnnotationID()
	}

	logger.Info("Generated annotation IDs for batch", "workspace_id", workspaceID, "annotation_ids", annotationIDs)

	// Queue each annotation individually to SQS
	successCount := 0
	successfulIDs := make([]string, 0, len(annotationIDs))

	logger.Info("Starting batch processing", "total_annotations", len(req.AnnotationRows), "workspace_id", workspaceID)

	for i, annotationRow := range req.AnnotationRows {
		annotationID := annotationIDs[i]

		logger.Debug("Processing annotation", "annotation_id", annotationID, "batch_index", i, "row_index", annotationRow.RowIndex)

		// Create individual annotation request for this row
		individualReq := api.CreateAnnotationRequest{
			WorkspaceID: req.WorkspaceID,
			FileHash:    req.FileHash,
			Options:     req.Options, // File options from batch request
			RowIndex:    annotationRow.RowIndex,
			Note:        req.Note,
			Color:       req.Color,
		}

		if err := createAnnotationDirect(ctx, workspaceID, email, annotationID, individualReq); err != nil {
			logger.Error("Failed to create individual annotation",
				"error", err,
				"annotation_id", annotationID,
				"batch_index", i,
				"row_index", annotationRow.RowIndex,
				"workspace_id", workspaceID,
				"file_hash", req.FileHash,
				"note_length", len(req.Note),
			)
			continue
		}
		logger.Debug("Successfully created annotation", "annotation_id", annotationID, "row_index", i)
		successCount++
		successfulIDs = append(successfulIDs, annotationID)
	}

	logger.Info("Individual annotations created", "workspace_id", workspaceID, "total_annotations", len(annotationIDs), "successful_annotations", successCount)

	// Return batch response with actual success/failure counts
	failureCount := len(annotationIDs) - successCount
	response := api.BatchAnnotationResponse{
		Message:       fmt.Sprintf("Batch of %d annotations created successfully (%d successful, %d failed)", len(annotationIDs), successCount, failureCount),
		WorkspaceID:   workspaceID,
		AnnotationIDs: successfulIDs, // Only return IDs that were successfully created
		SuccessCount:  successCount,
		FailureCount:  failureCount,
	}

	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 202,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func createAnnotationDirect(ctx context.Context, workspaceID, userEmail, annotationID string, req api.CreateAnnotationRequest) error {
	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339)

	logger.Debug("Creating annotation directly", "workspace_id", workspaceID, "annotation_id", annotationID)

	// Create annotation directly in DynamoDB
	annotation := api.Annotation{
		AnnotationID: annotationID,
		FileHash:     req.FileHash,
		Options:      req.Options, // File options (jpath, noHeaderRow, ingestTimezoneOverride)
		RowIndex:     req.RowIndex,
		Note:         req.Note,
		Color:        req.Color,
		CreatedBy:    userEmail,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt,
		Version:      1, // Set initial version
	}

	err := api.CreateAnnotation(ctx, workspaceID, annotation)
	if err != nil {
		logger.Error("Failed to create annotation",
			"error", err,
			"annotation_id", annotationID,
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
	auditDescription := fmt.Sprintf("%s added annotation %s to %s", userEmail, annotationID, workspaceID)
	if err := api.CreateAuditEntry(ctx, workspaceID, userEmail, auditDescription); err != nil {
		logger.Error("Failed to create audit entry",
			"error", err,
			"workspace_id", workspaceID,
			"annotation_id", annotationID,
		)
		// Don't fail the entire operation for audit failure
	}

	logger.Debug("Successfully created annotation directly",
		"annotation_id", annotationID,
		"workspace_id", workspaceID,
	)

	return nil
}

func main() {
	lambda.Start(handler)
}
