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
		Level: slog.LevelInfo,
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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(annotationUpdateHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func annotationUpdateHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Annotation update request received", "user_email", email)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Detect request type by trying to parse as batch first, then single
	var isBatch bool
	var batchReq api.BatchUpdateAnnotationRequest
	var singleReq api.UpdateAnnotationRequest
	annotationID := request.PathParameters["annotation_id"]

	// Try parsing as batch request first
	if err := json.Unmarshal([]byte(request.Body), &batchReq); err == nil && len(batchReq.Updates) > 0 {
		isBatch = true
		logger.Info("Detected batch annotation update request", "update_count", len(batchReq.Updates))
	} else if annotationID != "" {
		// Try parsing as single request (only if annotation_id is in path)
		if err := json.Unmarshal([]byte(request.Body), &singleReq); err != nil {
			logger.Error("Failed to parse request body as single update", "error", err, "body_length", len(request.Body))
			return api.CreateErrorResponse(400, "invalid_request", "Invalid request body")
		}
		// Set annotation ID from path for single request
		singleReq.AnnotationID = annotationID
		isBatch = false
		logger.Info("Detected single annotation update request", "annotation_id", annotationID)
	} else {
		logger.Error("Could not determine request type", "has_annotation_id", annotationID != "", "body_length", len(request.Body))
		return api.CreateErrorResponse(400, "invalid_request", "Invalid request: must be either single update with annotation_id in path or batch update")
	}

	// Common validation and permission checks
	if _, err := validateWorkspaceAndPermissions(ctx, workspaceID, email); err != nil {
		return *err, nil
	}

	if isBatch {
		return handleBatchUpdateRequest(ctx, workspaceID, email, batchReq)
	} else {
		return handleSingleUpdateRequest(ctx, workspaceID, email, singleReq)
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

	// Check write permissions
	logger.Debug("Checking write permissions", "workspace_id", workspaceID, "user_email", email)
	canWrite, err := api.CheckWritePermission(ctx, workspaceID, email, workspace.OwnerEmail)
	if err != nil {
		logger.Error("Failed to check write permissions", "error", err, "workspace_id", workspaceID, "user_email", email)
		response, _ := api.CreateErrorResponse(500, "internal_error", "Failed to check permissions")
		return nil, &response
	}

	if !canWrite {
		logger.Error("Write access denied", "workspace_id", workspaceID, "user_email", email)
		response, _ := api.CreateErrorResponse(403, "write_access_denied", "You do not have write access to this workspace")
		return nil, &response
	}

	return workspace, nil
}

func handleSingleUpdateRequest(ctx context.Context, workspaceID, email string, req api.UpdateAnnotationRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing single annotation update request", "annotation_id", req.AnnotationID, "note_length", len(req.Note), "color", req.Color)

	// Validate field sizes using size limits
	validator := api.NewRequestValidator()
	if !validator.ValidateUpdateAnnotationRequest(req) {
		logger.Error("Request validation failed", "errors", validator.GetErrorString())
		return api.CreateErrorResponse(400, "invalid_request", validator.GetErrorString())
	}

	// Validate required fields
	if req.AnnotationID == "" {
		logger.Error("Missing annotation ID")
		return api.CreateErrorResponse(400, "invalid_request", "Annotation ID is required")
	}
	if req.Note == "" && req.Color == "" {
		logger.Error("No fields to update")
		return api.CreateErrorResponse(400, "invalid_request", "At least one field (note or color) must be provided")
	}

	// Get existing annotation to verify it exists
	logger.Debug("Fetching existing annotation", "annotation_id", req.AnnotationID, "workspace_id", workspaceID)
	existingAnnotation, err := api.GetAnnotation(ctx, workspaceID, req.AnnotationID)
	if err != nil {
		logger.Error("Failed to get existing annotation", "error", err, "annotation_id", req.AnnotationID)
		return api.CreateErrorResponse(404, "annotation_not_found", "Annotation not found")
	}

	// Update annotation directly
	logger.Info("Updating annotation directly", "annotation_id", req.AnnotationID, "workspace_id", workspaceID)
	if err := updateAnnotationDirect(ctx, workspaceID, email, req.AnnotationID, req, existingAnnotation); err != nil {
		logger.Error("Failed to update annotation", "error", err, "annotation_id", req.AnnotationID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to update annotation")
	}

	logger.Info("Single annotation updated successfully", "annotation_id", req.AnnotationID, "workspace_id", workspaceID)

	// Return success response
	response := map[string]interface{}{
		"message":       "Annotation updated successfully",
		"annotation_id": req.AnnotationID,
		"workspace_id":  workspaceID,
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

func handleBatchUpdateRequest(ctx context.Context, workspaceID, email string, req api.BatchUpdateAnnotationRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing batch annotation update request", "update_count", len(req.Updates), "workspace_id", workspaceID)

	// Validate required fields
	if len(req.Updates) == 0 {
		logger.Error("Missing updates for batch request")
		return api.CreateErrorResponse(400, "invalid_request", "updates (non-empty) is required")
	}

	// Validate batch size limit (max 256)
	if len(req.Updates) > 256 {
		logger.Error("Batch size exceeds limit", "update_count", len(req.Updates), "max_allowed", 256)
		return api.CreateErrorResponse(400, "batch_size_exceeded", "Maximum batch size is 256 updates")
	}

	// Validate each update request and field sizes
	validator := api.NewRequestValidator()
	for i, update := range req.Updates {
		if update.AnnotationID == "" {
			logger.Error("Invalid update request", "update_index", i, "missing_annotation_id", true)
			return api.CreateErrorResponse(400, "invalid_request", fmt.Sprintf("updates[%d] must have annotation_id", i))
		}
		if update.Note == "" && update.Color == "" {
			logger.Error("Invalid update request", "update_index", i, "no_fields_to_update", true)
			return api.CreateErrorResponse(400, "invalid_request", fmt.Sprintf("updates[%d] must have at least one field (note or color) to update", i))
		}

		// Validate field sizes for each update
		if !validator.ValidateUpdateAnnotationRequest(update) {
			logger.Error("Batch update validation failed", "update_index", i, "errors", validator.GetErrorString())
			return api.CreateErrorResponse(400, "invalid_request", fmt.Sprintf("updates[%d]: %s", i, validator.GetErrorString()))
		}
	}

	// Process each update individually
	successCount := 0
	successfulIDs := make([]string, 0, len(req.Updates))

	logger.Info("Starting batch update processing", "total_updates", len(req.Updates), "workspace_id", workspaceID)

	for i, update := range req.Updates {
		logger.Debug("Processing update", "annotation_id", update.AnnotationID, "update_index", i, "note_length", len(update.Note), "color", update.Color)

		// Get existing annotation to verify it exists
		existingAnnotation, err := api.GetAnnotation(ctx, workspaceID, update.AnnotationID)
		if err != nil {
			logger.Error("Failed to get existing annotation for batch update",
				"error", err,
				"annotation_id", update.AnnotationID,
				"update_index", i,
				"workspace_id", workspaceID,
			)
			continue
		}

		if err := updateAnnotationDirect(ctx, workspaceID, email, update.AnnotationID, update, existingAnnotation); err != nil {
			logger.Error("Failed to update individual annotation",
				"error", err,
				"annotation_id", update.AnnotationID,
				"update_index", i,
				"workspace_id", workspaceID,
				"note_length", len(update.Note),
				"color", update.Color,
			)
			continue
		}
		logger.Debug("Successfully updated annotation", "annotation_id", update.AnnotationID, "update_index", i)
		successCount++
		successfulIDs = append(successfulIDs, update.AnnotationID)
	}

	logger.Info("Individual annotation updates completed", "workspace_id", workspaceID, "total_updates", len(req.Updates), "successful_updates", successCount)

	// Return batch response with actual success/failure counts
	failureCount := len(req.Updates) - successCount
	response := api.BatchUpdateAnnotationResponse{
		Message:      fmt.Sprintf("Batch of %d annotation updates processed (%d successful, %d failed)", len(req.Updates), successCount, failureCount),
		WorkspaceID:  workspaceID,
		UpdatedIDs:   successfulIDs, // Only return IDs that were successfully updated
		SuccessCount: successCount,
		FailureCount: failureCount,
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

func updateAnnotationDirect(ctx context.Context, workspaceID, userEmail, annotationID string, req api.UpdateAnnotationRequest, existingAnnotation *api.Annotation) error {
	logger.Debug("Updating annotation directly", "workspace_id", workspaceID, "annotation_id", annotationID)

	// Update annotation directly in DynamoDB
	note := existingAnnotation.Note
	color := string(existingAnnotation.Color)

	if req.Note != "" {
		note = req.Note
	}
	if req.Color != "" {
		color = string(req.Color)
	}

	err := api.UpdateAnnotation(ctx, workspaceID, annotationID, note, color, 1)
	if err != nil {
		logger.Error("Failed to update annotation",
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
	auditDescription := fmt.Sprintf("%s updated annotation %s in %s", userEmail, annotationID, workspaceID)
	if err := api.CreateAuditEntry(ctx, workspaceID, userEmail, auditDescription); err != nil {
		logger.Error("Failed to create audit entry",
			"error", err,
			"workspace_id", workspaceID,
			"annotation_id", annotationID,
		)
		// Don't fail the entire operation for audit failure
	}

	logger.Debug("Successfully updated annotation directly",
		"annotation_id", annotationID,
		"workspace_id", workspaceID,
	)

	return nil
}

func main() {
	lambda.Start(handler)
}
