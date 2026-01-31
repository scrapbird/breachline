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
	rateLimitedHandler := api.ApplyRateLimitingToHandler(annotationDeleteHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func annotationDeleteHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Get user context from authorizer
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Annotation delete request received", "user_email", email)

	// Get workspace ID from path
	workspaceID := request.PathParameters["workspace_id"]
	logger.Info("Request parameters", "workspace_id", workspaceID)

	if workspaceID == "" {
		logger.Error("Missing workspace ID")
		return api.CreateErrorResponse(400, "invalid_request", "Workspace ID is required")
	}

	// Detect request type by checking if annotation_id is in path (single) or body has annotation_ids (batch)
	annotationID := request.PathParameters["annotation_id"]
	var isBatch bool
	var batchReq api.BatchDeleteAnnotationRequest

	if annotationID == "" {
		// No annotation_id in path, try parsing as batch request
		if err := json.Unmarshal([]byte(request.Body), &batchReq); err == nil && len(batchReq.AnnotationIDs) > 0 {
			isBatch = true
			logger.Info("Detected batch annotation delete request", "annotation_count", len(batchReq.AnnotationIDs))
		} else {
			logger.Error("Missing annotation ID in path and invalid batch request body", "body_length", len(request.Body))
			return api.CreateErrorResponse(400, "invalid_request", "Annotation ID is required in path for single deletion or valid annotation_ids array in body for batch deletion")
		}
	} else {
		isBatch = false
		logger.Info("Detected single annotation delete request", "annotation_id", annotationID)
	}

	// Common validation and permission checks
	_, err := validateWorkspaceAndPermissions(ctx, workspaceID, email)
	if err != nil {
		return *err, nil
	}

	if isBatch {
		return handleBatchDeleteRequest(ctx, workspaceID, email, batchReq)
	} else {
		return handleSingleDeleteRequest(ctx, workspaceID, email, annotationID)
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

func handleSingleDeleteRequest(ctx context.Context, workspaceID, email, annotationID string) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing single annotation delete request", "annotation_id", annotationID)

	// Delete annotation directly
	logger.Info("Deleting annotation directly", "workspace_id", workspaceID, "annotation_id", annotationID, "user_email", email)
	if err := deleteAnnotationDirect(ctx, workspaceID, email, annotationID); err != nil {
		logger.Error("Failed to delete annotation", "error", err, "workspace_id", workspaceID, "annotation_id", annotationID)
		return api.CreateErrorResponse(500, "internal_error", "Failed to delete annotation")
	}
	logger.Info("Single annotation deleted successfully", "workspace_id", workspaceID, "annotation_id", annotationID)

	// Return response
	response := api.AnnotationResponse{
		Message:      "Annotation deleted successfully",
		AnnotationID: annotationID,
		WorkspaceID:  workspaceID,
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

func handleBatchDeleteRequest(ctx context.Context, workspaceID, email string, req api.BatchDeleteAnnotationRequest) (events.APIGatewayProxyResponse, error) {
	logger.Info("Processing batch annotation delete request", "annotation_count", len(req.AnnotationIDs))

	// Validate required fields
	if len(req.AnnotationIDs) == 0 {
		logger.Error("Missing required fields for batch delete request",
			"annotation_ids_count", len(req.AnnotationIDs))
		return api.CreateErrorResponse(400, "invalid_request", "annotation_ids (non-empty) is required")
	}

	// Validate batch size limit (max 256)
	if len(req.AnnotationIDs) > 256 {
		logger.Error("Batch size exceeds limit", "annotation_count", len(req.AnnotationIDs), "max_allowed", 256)
		return api.CreateErrorResponse(400, "batch_size_exceeded", "Maximum batch size is 256 annotations")
	}

	// Validate each annotation ID
	for i, annotationID := range req.AnnotationIDs {
		if annotationID == "" {
			logger.Error("Invalid annotation ID", "annotation_index", i, "annotation_id", annotationID)
			return api.CreateErrorResponse(400, "invalid_request", fmt.Sprintf("annotation_ids[%d] must be non-empty", i))
		}
	}

	logger.Info("Validated annotation IDs for batch delete", "workspace_id", workspaceID, "annotation_ids", req.AnnotationIDs)

	// Delete each annotation directly
	var successCount int
	var successfulIDs []string

	for _, annotationID := range req.AnnotationIDs {
		if err := deleteAnnotationDirect(ctx, workspaceID, email, annotationID); err != nil {
			logger.Error("Failed to delete individual annotation", "error", err, "annotation_id", annotationID)
			continue
		}
		successCount++
		successfulIDs = append(successfulIDs, annotationID)
	}

	logger.Info("Individual annotations deleted", "workspace_id", workspaceID, "total_annotations", len(req.AnnotationIDs), "successful_annotations", successCount)

	// Return batch response with actual success/failure counts
	failureCount := len(req.AnnotationIDs) - successCount
	response := api.BatchDeleteAnnotationResponse{
		Message:      fmt.Sprintf("Batch of %d annotations deleted successfully (%d successful, %d failed)", len(req.AnnotationIDs), successCount, failureCount),
		WorkspaceID:  workspaceID,
		DeletedIDs:   successfulIDs, // Only return IDs that were successfully deleted
		SuccessCount: successCount,
		FailureCount: failureCount,
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

func deleteAnnotationDirect(ctx context.Context, workspaceID, userEmail, annotationID string) error {
	logger.Debug("Deleting annotation directly", "workspace_id", workspaceID, "annotation_id", annotationID)

	// Delete annotation directly from DynamoDB
	err := api.DeleteAnnotation(ctx, workspaceID, annotationID)
	if err != nil {
		logger.Error("Failed to delete annotation",
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
	auditDescription := fmt.Sprintf("%s deleted annotation %s from %s", userEmail, annotationID, workspaceID)
	if err := api.CreateAuditEntry(ctx, workspaceID, userEmail, auditDescription); err != nil {
		logger.Error("Failed to create audit entry",
			"error", err,
			"workspace_id", workspaceID,
			"annotation_id", annotationID,
		)
		// Don't fail the entire operation for audit failure
	}

	logger.Debug("Successfully deleted annotation directly",
		"annotation_id", annotationID,
		"workspace_id", workspaceID,
	)

	return nil
}

func main() {
	lambda.Start(handler)
}
