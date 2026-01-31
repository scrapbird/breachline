package api

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// AuditEntry represents an audit log entry for workspace changes
type AuditEntry struct {
	AuditID     string `dynamodbav:"audit_id" json:"audit_id"`
	WorkspaceID string `dynamodbav:"workspace_id" json:"workspace_id"`
	Email       string `dynamodbav:"email" json:"email"`
	Description string `dynamodbav:"description" json:"description"`
	CreatedAt   string `dynamodbav:"created_at" json:"created_at"`
	TTL         int64  `dynamodbav:"ttl" json:"ttl"`
}

type ListAuditEntriesRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Limit       int32  `json:"limit,omitempty"`
}

type ListAuditEntriesResponse struct {
	AuditEntries []AuditEntry `json:"audit_entries"`
}

// CreateAuditEntry creates a new audit log entry for workspace changes
func CreateAuditEntry(ctx context.Context, workspaceID, email, description string) error {
	auditID := GenerateAuditID()
	now := time.Now().UTC()
	createdAt := now.Format(time.RFC3339)
	
	// Set TTL to 90 days from now
	ttl := now.AddDate(0, 0, 90).Unix()

	auditEntry := AuditEntry{
		AuditID:     auditID,
		WorkspaceID: workspaceID,
		Email:       email,
		Description: description,
		CreatedAt:   createdAt,
		TTL:         ttl,
	}

	logger.Debug("Creating audit entry",
		"audit_id", auditID,
		"workspace_id", workspaceID,
		"email", email,
		"description", description,
	)

	item, err := attributevalue.MarshalMap(auditEntry)
	if err != nil {
		logger.Error("Failed to marshal audit entry",
			"error", err,
			"audit_id", auditID,
		)
		return err
	}

	_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &auditTable,
		Item:      item,
	})
	if err != nil {
		logger.Error("Failed to put audit entry",
			"error", err,
			"audit_id", auditID,
			"table", auditTable,
		)
		return err
	}

	logger.Debug("Successfully created audit entry",
		"audit_id", auditID,
		"workspace_id", workspaceID,
	)

	return nil
}

// GetAuditEntries retrieves audit entries for a workspace
func GetAuditEntries(ctx context.Context, workspaceID string, limit int32) ([]AuditEntry, error) {
	logger.Debug("Querying audit entries from DynamoDB", 
		"workspace_id", workspaceID, 
		"limit", limit,
	)

	input := &dynamodb.QueryInput{
		TableName:              &auditTable,
		IndexName:              aws.String("workspace-audit-index"),
		KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
		ScanIndexForward: aws.Bool(false), // Sort descending by created_at (most recent first)
	}

	if limit > 0 {
		input.Limit = aws.Int32(limit)
	}

	result, err := ddbClient.Query(ctx, input)
	if err != nil {
		logger.Error("Failed to query audit entries",
			"error", err,
			"workspace_id", workspaceID,
		)
		return nil, err
	}

	var auditEntries []AuditEntry
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &auditEntries); err != nil {
		logger.Error("Failed to unmarshal audit entries",
			"error", err,
			"workspace_id", workspaceID,
		)
		return nil, err
	}

	logger.Debug("Successfully retrieved audit entries",
		"workspace_id", workspaceID,
		"count", len(auditEntries),
	)

	return auditEntries, nil
}

// UpdateWorkspaceTimestamp updates the workspace's updated_at timestamp
func UpdateWorkspaceTimestamp(ctx context.Context, workspaceID string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	logger.Debug("Updating workspace timestamp",
		"workspace_id", workspaceID,
		"updated_at", now,
	)

	_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &workspacesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
		UpdateExpression: aws.String("SET updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":updated_at": &types.AttributeValueMemberS{Value: now},
		},
	})

	if err != nil {
		logger.Error("Failed to update workspace timestamp",
			"error", err,
			"workspace_id", workspaceID,
			"table", workspacesTable,
		)
		return err
	}

	logger.Debug("Successfully updated workspace timestamp",
		"workspace_id", workspaceID,
		"updated_at", now,
	)

	return nil
}

// GenerateAuditID generates a unique audit ID
func GenerateAuditID() string {
	return fmt.Sprintf("audit_%s", uuid.New().String())
}
