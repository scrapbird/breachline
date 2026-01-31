package api

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// ColumnHashes is DEPRECATED - kept for backward compatibility with existing annotations
type ColumnHashes []map[string]string

// UnmarshalDynamoDBAttributeValue implements custom unmarshaling for ColumnHashes
// to handle JSON-encoded strings stored in DynamoDB
func (c *ColumnHashes) UnmarshalDynamoDBAttributeValue(av types.AttributeValue) error {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		// Parse JSON string back into ColumnHashes
		return json.Unmarshal([]byte(v.Value), c)
	default:
		return fmt.Errorf("unexpected attribute type for ColumnHashes: %T", av)
	}
}

type AnnotationColor string

const (
	AnnotationColorWhite  AnnotationColor = "white"
	AnnotationColorGrey   AnnotationColor = "grey"
	AnnotationColorBlue   AnnotationColor = "blue"
	AnnotationColorGreen  AnnotationColor = "green"
	AnnotationColorYellow AnnotationColor = "yellow"
	AnnotationColorOrange AnnotationColor = "orange"
	AnnotationColorRed    AnnotationColor = "red"
)

// Annotation represents a single row annotation
type Annotation struct {
	WorkspaceID  string          `json:"workspace_id" dynamodbav:"workspace_id"`
	AnnotationID string          `json:"annotation_id" dynamodbav:"annotation_id"`
	FileHash     string          `json:"file_hash" dynamodbav:"file_hash"`
	Options      FileOptions     `json:"options" dynamodbav:"options"`                                 // File options (jpath, noHeaderRow, ingestTimezoneOverride)
	RowIndex     int             `json:"row_index" dynamodbav:"row_index"`                             // 0-based index of the annotated row in the source file
	ColumnHashes ColumnHashes    `json:"column_hashes,omitempty" dynamodbav:"column_hashes,omitempty"` // DEPRECATED: kept for backward compatibility
	Note         string          `json:"note" dynamodbav:"note"`
	Color        AnnotationColor `json:"color" dynamodbav:"color"` // grey, blue, yellow, green, orange, red
	CreatedBy    string          `json:"created_by" dynamodbav:"created_by"`
	CreatedAt    string          `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt    string          `json:"updated_at" dynamodbav:"updated_at"`
	Version      int64           `json:"version" dynamodbav:"version"`
}

type CreateAnnotationRequest struct {
	WorkspaceID  string              `json:"workspace_id"`
	FileHash     string              `json:"file_hash"`
	Options      FileOptions         `json:"options"`                 // File options (jpath, noHeaderRow, ingestTimezoneOverride)
	RowIndex     int                 `json:"row_index"`               // 0-based index of the annotated row
	ColumnHashes []map[string]string `json:"column_hashes,omitempty"` // DEPRECATED: kept for backward compatibility
	Note         string              `json:"note"`
	Color        AnnotationColor     `json:"color"`
}

type UpdateAnnotationRequest struct {
	AnnotationID string          `json:"annotation_id"`
	Note         string          `json:"note"`
	Color        AnnotationColor `json:"color"`
}

// BatchUpdateAnnotationRequest represents a request to update multiple annotations
type BatchUpdateAnnotationRequest struct {
	WorkspaceID string                    `json:"workspace_id"`
	Updates     []UpdateAnnotationRequest `json:"updates"`
}

// BatchUpdateAnnotationResponse represents the response from a batch annotation update
type BatchUpdateAnnotationResponse struct {
	Message      string   `json:"message"`
	WorkspaceID  string   `json:"workspace_id"`
	UpdatedIDs   []string `json:"updated_ids"`
	SuccessCount int      `json:"success_count"`
	FailureCount int      `json:"failure_count"`
}

type DeleteAnnotationRequest struct {
	AnnotationID string `json:"annotation_id"`
}

// BatchDeleteAnnotationRequest represents a request to delete multiple annotations
type BatchDeleteAnnotationRequest struct {
	WorkspaceID   string   `json:"workspace_id"`
	AnnotationIDs []string `json:"annotation_ids"`
}

// BatchDeleteAnnotationResponse represents the response from a batch annotation deletion
type BatchDeleteAnnotationResponse struct {
	Message      string   `json:"message"`
	WorkspaceID  string   `json:"workspace_id"`
	DeletedIDs   []string `json:"deleted_ids"`
	SuccessCount int      `json:"success_count"`
	FailureCount int      `json:"failure_count"`
}

type ListAnnotationsResponse struct {
	Annotations []Annotation `json:"annotations"`
}

type AnnotationResponse struct {
	Message      string `json:"message"`
	AnnotationID string `json:"annotation_id"`
	WorkspaceID  string `json:"workspace_id"`
}

// AnnotationRow represents a single row in a batch annotation request
type AnnotationRow struct {
	RowIndex     int                 `json:"row_index"`               // 0-based index of the annotated row
	ColumnHashes []map[string]string `json:"column_hashes,omitempty"` // DEPRECATED: kept for backward compatibility
}

// BatchCreateAnnotationRequest represents a request to create multiple annotations for the same file
type BatchCreateAnnotationRequest struct {
	WorkspaceID    string          `json:"workspace_id"`
	FileHash       string          `json:"file_hash"`
	Options        FileOptions     `json:"options"` // File options (jpath, noHeaderRow, ingestTimezoneOverride)
	Note           string          `json:"note"`
	Color          AnnotationColor `json:"color"`
	AnnotationRows []AnnotationRow `json:"annotation_rows"`
}

// BatchAnnotationResponse represents the response from a batch annotation creation
type BatchAnnotationResponse struct {
	Message       string   `json:"message"`
	WorkspaceID   string   `json:"workspace_id"`
	AnnotationIDs []string `json:"annotation_ids"`
	SuccessCount  int      `json:"success_count"`
	FailureCount  int      `json:"failure_count"`
}

func CreateAnnotation(ctx context.Context, workspaceID string, annotation Annotation) error {
	// Validate input data
	if workspaceID == "" {
		logger.Error("Empty workspace ID", "annotation_id", annotation.AnnotationID)
		return fmt.Errorf("workspace ID cannot be empty")
	}
	if annotation.AnnotationID == "" {
		logger.Error("Empty annotation ID", "workspace_id", workspaceID)
		return fmt.Errorf("annotation ID cannot be empty")
	}
	// RowIndex validation: must be non-negative (0 is valid for first row)
	if annotation.RowIndex < 0 {
		logger.Error("Invalid row index", "annotation_id", annotation.AnnotationID, "workspace_id", workspaceID, "row_index", annotation.RowIndex)
		return fmt.Errorf("row index cannot be negative")
	}

	// Check item size (DynamoDB has 400KB limit per item)
	itemSizeEstimate := len(workspaceID) + len(annotation.AnnotationID) + len(annotation.FileHash) +
		len(annotation.Options.JPath) + len(annotation.Note) + len(annotation.Color) +
		len(annotation.CreatedBy) + len(annotation.CreatedAt) + len(annotation.UpdatedAt) + 100 // overhead
	if itemSizeEstimate > 350000 { // 350KB threshold
		logger.Error("Annotation item too large",
			"annotation_id", annotation.AnnotationID,
			"estimated_size", itemSizeEstimate,
			"note_size", len(annotation.Note),
		)
		return fmt.Errorf("annotation item size (%d bytes) exceeds DynamoDB limits", itemSizeEstimate)
	}

	item := map[string]types.AttributeValue{
		"workspace_id":  &types.AttributeValueMemberS{Value: workspaceID},
		"annotation_id": &types.AttributeValueMemberS{Value: annotation.AnnotationID},
		"file_hash":     &types.AttributeValueMemberS{Value: annotation.FileHash},
		"row_index":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", annotation.RowIndex)},
		"note":          &types.AttributeValueMemberS{Value: annotation.Note},
		"color":         &types.AttributeValueMemberS{Value: string(annotation.Color)},
		"created_by":    &types.AttributeValueMemberS{Value: annotation.CreatedBy},
		"created_at":    &types.AttributeValueMemberS{Value: annotation.CreatedAt},
		"updated_at":    &types.AttributeValueMemberS{Value: annotation.UpdatedAt},
		"version":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", annotation.Version)},
	}

	// Add options as nested map
	optionsItem, err := attributevalue.MarshalMap(annotation.Options)
	if err != nil {
		logger.Error("Failed to marshal options", "error", err)
		return err
	}
	item["options"] = &types.AttributeValueMemberM{Value: optionsItem}

	logger.Info("Attempting to put annotation item to DynamoDB",
		"annotation_id", annotation.AnnotationID,
		"workspace_id", workspaceID,
		"row_index", annotation.RowIndex,
		"table", annotationsTable,
		"item_size_estimate", itemSizeEstimate,
	)

	putItemInput := &dynamodb.PutItemInput{
		TableName: &annotationsTable,
		Item:      item,
	}

	result, err := ddbClient.PutItem(ctx, putItemInput)

	if err != nil {
		logger.Error("Failed to create Annotation item",
			"error", err,
			"error_type", fmt.Sprintf("%T", err),
			"annotation_id", annotation.AnnotationID,
			"workspace_id", workspaceID,
			"table", annotationsTable,
			"item_size_estimate", itemSizeEstimate,
		)
		return err
	}

	logger.Info("Successfully created annotation item in DynamoDB",
		"annotation_id", annotation.AnnotationID,
		"workspace_id", workspaceID,
		"consumed_capacity", result.ConsumedCapacity,
	)

	return nil
}

func UpdateAnnotation(ctx context.Context, workspaceID string, annotationID string, note string, color string, version int64) error {
	logger.Info("Updating annotation",
		"annotation_id", annotationID,
		"workspace_id", workspaceID,
		"note", note,
		"color", color,
	)

	updateExpr := "SET note = :note, color = :color, updated_at = :updated_at, version = :version"
	exprValues := map[string]types.AttributeValue{
		":note":       &types.AttributeValueMemberS{Value: note},
		":color":      &types.AttributeValueMemberS{Value: string(color)},
		":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		":version":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", version)},
	}

	logger.Debug("Executing DynamoDB update",
		"annotation_id", annotationID,
		"table", annotationsTable,
	)

	_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &annotationsTable,
		Key: map[string]types.AttributeValue{
			"workspace_id":  &types.AttributeValueMemberS{Value: workspaceID},
			"annotation_id": &types.AttributeValueMemberS{Value: annotationID},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprValues,
	})
	if err != nil {
		logger.Error("Failed to update annotation",
			"error", err,
			"annotation_id", annotationID,
			"workspace_id", workspaceID,
			"table", annotationsTable,
		)
		return err
	}

	return nil
}

func GetAnnotation(ctx context.Context, workspaceID, annotationID string) (*Annotation, error) {
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &annotationsTable,
		Key: map[string]types.AttributeValue{
			"workspace_id":  &types.AttributeValueMemberS{Value: workspaceID},
			"annotation_id": &types.AttributeValueMemberS{Value: annotationID},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("annotation not found")
	}

	var annotation Annotation
	if err := attributevalue.UnmarshalMap(result.Item, &annotation); err != nil {
		return nil, err
	}

	return &annotation, nil
}

func GetWorkspaceAnnotations(ctx context.Context, workspaceID, fileHash, annotationColor string) ([]Annotation, error) {
	var allAnnotations []Annotation
	var lastEvaluatedKey map[string]types.AttributeValue

	for {
		var result *dynamodb.QueryOutput
		var err error

		// Build query input based on filters
		var queryInput *dynamodb.QueryInput
		if fileHash != "" {
			// Query by file hash using GSI
			queryInput = &dynamodb.QueryInput{
				TableName:              &annotationsTable,
				IndexName:              aws.String("file-hash-index"),
				KeyConditionExpression: aws.String("workspace_id = :workspace_id AND file_hash = :file_hash"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
					":file_hash":    &types.AttributeValueMemberS{Value: fileHash},
				},
			}
		} else if annotationColor != "" {
			// Query by type using GSI
			queryInput = &dynamodb.QueryInput{
				TableName:              &annotationsTable,
				IndexName:              aws.String("type-index"),
				KeyConditionExpression: aws.String("workspace_id = :workspace_id AND color = :type"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
					":color":        &types.AttributeValueMemberS{Value: annotationColor},
				},
			}
		} else {
			// Query all annotations for workspace
			queryInput = &dynamodb.QueryInput{
				TableName:              &annotationsTable,
				KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
				},
			}
		}

		// Set pagination key if we have one from previous page
		if lastEvaluatedKey != nil {
			queryInput.ExclusiveStartKey = lastEvaluatedKey
		}

		// Execute the query
		result, err = ddbClient.Query(ctx, queryInput)
		if err != nil {
			return nil, err
		}

		// Unmarshal this page of annotations
		var pageAnnotations []Annotation
		if err := attributevalue.UnmarshalListOfMaps(result.Items, &pageAnnotations); err != nil {
			return nil, err
		}

		// Add this page to our total results
		allAnnotations = append(allAnnotations, pageAnnotations...)

		// Check if there are more pages
		lastEvaluatedKey = result.LastEvaluatedKey
		if lastEvaluatedKey == nil {
			// No more pages, we're done
			break
		}
	}

	return allAnnotations, nil
}

func DeleteAnnotation(ctx context.Context, workspaceID, annotationID string) error {
	logger.Debug("Deleting annotation",
		"annotation_id", annotationID,
		"workspace_id", workspaceID,
	)

	_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &annotationsTable,
		Key: map[string]types.AttributeValue{
			"workspace_id":  &types.AttributeValueMemberS{Value: workspaceID},
			"annotation_id": &types.AttributeValueMemberS{Value: annotationID},
		},
	})
	if err != nil {
		logger.Error("Failed to delete annotation",
			"error", err,
			"annotation_id", annotationID,
			"workspace_id", workspaceID,
		)
		return err
	}

	return nil
}

// DeleteAnnotationsByFileHash deletes all annotations for a given file hash in a workspace
func DeleteAnnotationsByFileHash(ctx context.Context, workspaceID, fileHash string) (int, error) {
	logger.Debug("Deleting all annotations for file hash",
		"file_hash", fileHash,
		"workspace_id", workspaceID,
	)

	// Get all annotations for this file hash
	annotations, err := GetWorkspaceAnnotations(ctx, workspaceID, fileHash, "")
	if err != nil {
		logger.Error("Failed to get annotations for deletion",
			"error", err,
			"file_hash", fileHash,
			"workspace_id", workspaceID,
		)
		return 0, err
	}

	// Delete each annotation
	deleteCount := 0
	for _, annot := range annotations {
		if err := DeleteAnnotation(ctx, workspaceID, annot.AnnotationID); err != nil {
			logger.Error("Failed to delete annotation",
				"error", err,
				"annotation_id", annot.AnnotationID,
				"workspace_id", workspaceID,
			)
			// Continue deleting other annotations even if one fails
		} else {
			deleteCount++
		}
	}

	logger.Info("Deleted annotations for file hash",
		"file_hash", fileHash,
		"workspace_id", workspaceID,
		"delete_count", deleteCount,
	)

	return deleteCount, nil
}

// DeleteAnnotationsByFileVariant deletes annotations for a specific file variant
// This matches annotations by file_hash, jpath, no_header_row, and ingest_timezone_override
func DeleteAnnotationsByFileVariant(ctx context.Context, workspaceID, fileHash, jpath string, noHeaderRow bool, ingestTzOverride string) (int, error) {
	logger.Debug("Deleting annotations for specific file variant",
		"file_hash", fileHash,
		"jpath", jpath,
		"no_header_row", noHeaderRow,
		"ingest_timezone_override", ingestTzOverride,
		"workspace_id", workspaceID,
	)

	// Normalize ingestTzOverride for comparison
	if ingestTzOverride == "" {
		ingestTzOverride = "default"
	}

	// Get all annotations for this file hash
	annotations, err := GetWorkspaceAnnotations(ctx, workspaceID, fileHash, "")
	if err != nil {
		logger.Error("Failed to get annotations for deletion",
			"error", err,
			"file_hash", fileHash,
			"workspace_id", workspaceID,
		)
		return 0, err
	}

	// Delete only annotations matching the specific file variant
	deleteCount := 0
	for _, annot := range annotations {
		// Normalize annotation's ingestTzOverride for comparison
		annotTzOverride := annot.Options.IngestTimezoneOverride
		if annotTzOverride == "" {
			annotTzOverride = "default"
		}

		// Check if this annotation matches the specific file variant
		if annot.Options.JPath == jpath && annot.Options.NoHeaderRow == noHeaderRow && annotTzOverride == ingestTzOverride {
			if err := DeleteAnnotation(ctx, workspaceID, annot.AnnotationID); err != nil {
				logger.Error("Failed to delete annotation",
					"error", err,
					"annotation_id", annot.AnnotationID,
					"workspace_id", workspaceID,
				)
				// Continue deleting other annotations even if one fails
			} else {
				deleteCount++
			}
		}
	}

	logger.Info("Deleted annotations for file variant",
		"file_hash", fileHash,
		"jpath", jpath,
		"no_header_row", noHeaderRow,
		"ingest_timezone_override", ingestTzOverride,
		"workspace_id", workspaceID,
		"delete_count", deleteCount,
	)

	return deleteCount, nil
}

func GenerateAnnotationID() string {
	return fmt.Sprintf("ann_%s", uuid.New().String())
}
