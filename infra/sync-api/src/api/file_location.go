package api

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// WorkspaceFileLocation represents a file location for a specific Breachline instance
type WorkspaceFileLocation struct {
	InstanceID  string `json:"instance_id" dynamodbav:"instance_id"`
	FileHash    string `json:"file_hash" dynamodbav:"file_hash"`
	WorkspaceID string `json:"workspace_id" dynamodbav:"workspace_id"`
	FilePath    string `json:"file_path" dynamodbav:"file_path"` // Absolute path on the instance
	CreatedAt   string `json:"created_at" dynamodbav:"created_at"`
	UpdatedAt   string `json:"updated_at" dynamodbav:"updated_at"`
}

type StoreFileLocationRequest struct {
	InstanceID  string `json:"instance_id"`
	FileHash    string `json:"file_hash"`
	WorkspaceID string `json:"workspace_id"`
	FilePath    string `json:"file_path"`
}

type GetFileLocationRequest struct {
	InstanceID string `json:"instance_id"`
	FileHash   string `json:"file_hash"`
}

type FileLocationResponse struct {
	Message  string `json:"message"`
	FilePath string `json:"file_path,omitempty"`
}

type ListFileLocationsRequest struct {
	InstanceID string `json:"instance_id"`
}

type ListFileLocationsResponse struct {
	FileLocations []WorkspaceFileLocation `json:"file_locations"`
}

// StoreFileLocation stores or updates a file location for a specific instance
func StoreFileLocation(ctx context.Context, location WorkspaceFileLocation) error {
	logger.Info("Storing file location",
		"instance_id", location.InstanceID,
		"file_hash", location.FileHash,
		"workspace_id", location.WorkspaceID,
		"file_path", location.FilePath,
	)

	// Set timestamps
	now := time.Now().Format(time.RFC3339)
	location.CreatedAt = now
	location.UpdatedAt = now

	item, err := attributevalue.MarshalMap(location)
	if err != nil {
		logger.Error("Failed to marshal file location",
			"error", err,
			"instance_id", location.InstanceID,
			"file_hash", location.FileHash,
		)
		return err
	}

	logger.Debug("Putting file location item to DynamoDB",
		"instance_id", location.InstanceID,
		"file_hash", location.FileHash,
		"table", fileLocationsTable,
	)

	_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &fileLocationsTable,
		Item:      item,
	})

	if err != nil {
		logger.Error("Failed to store file location",
			"error", err,
			"instance_id", location.InstanceID,
			"file_hash", location.FileHash,
			"table", fileLocationsTable,
		)
		return err
	}

	return nil
}

// GetFileLocation retrieves a file location for a specific instance and file hash
func GetFileLocation(ctx context.Context, instanceID string, fileHash string) (*WorkspaceFileLocation, error) {
	logger.Debug("Getting file location",
		"instance_id", instanceID,
		"file_hash", fileHash,
	)

	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &fileLocationsTable,
		Key: map[string]types.AttributeValue{
			"instance_id": &types.AttributeValueMemberS{Value: instanceID},
			"file_hash":   &types.AttributeValueMemberS{Value: fileHash},
		},
	})
	if err != nil {
		logger.Error("Failed to get file location",
			"error", err,
			"instance_id", instanceID,
			"file_hash", fileHash,
		)
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("file location not found")
	}

	var location WorkspaceFileLocation
	if err := attributevalue.UnmarshalMap(result.Item, &location); err != nil {
		logger.Error("Failed to unmarshal file location",
			"error", err,
			"instance_id", instanceID,
			"file_hash", fileHash,
		)
		return nil, err
	}

	return &location, nil
}

// ListFileLocationsByInstance retrieves all file locations for a specific instance
func ListFileLocationsByInstance(ctx context.Context, instanceID string) ([]WorkspaceFileLocation, error) {
	logger.Debug("Listing file locations for instance",
		"instance_id", instanceID,
	)

	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &fileLocationsTable,
		KeyConditionExpression: aws.String("instance_id = :instance_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":instance_id": &types.AttributeValueMemberS{Value: instanceID},
		},
	})

	if err != nil {
		logger.Error("Failed to list file locations",
			"error", err,
			"instance_id", instanceID,
		)
		return nil, err
	}

	var locations []WorkspaceFileLocation
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &locations); err != nil {
		logger.Error("Failed to unmarshal file locations",
			"error", err,
			"instance_id", instanceID,
		)
		return nil, err
	}

	return locations, nil
}

// DeleteFileLocation removes a file location for a specific instance and file hash
func DeleteFileLocation(ctx context.Context, instanceID string, fileHash string) error {
	logger.Debug("Deleting file location",
		"instance_id", instanceID,
		"file_hash", fileHash,
	)

	_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &fileLocationsTable,
		Key: map[string]types.AttributeValue{
			"instance_id": &types.AttributeValueMemberS{Value: instanceID},
			"file_hash":   &types.AttributeValueMemberS{Value: fileHash},
		},
	})
	if err != nil {
		logger.Error("Failed to delete file location",
			"error", err,
			"instance_id", instanceID,
			"file_hash", fileHash,
		)
		return err
	}

	return nil
}

// DeleteFileLocationsByFileHash removes all file locations for a specific file hash in a workspace
// This is called when a file is deleted from a workspace
func DeleteFileLocationsByFileHash(ctx context.Context, workspaceID string, fileHash string) (int, error) {
	logger.Info("Deleting all file locations for file hash",
		"workspace_id", workspaceID,
		"file_hash", fileHash,
	)

	// Query using the GSI to find all file locations for this workspace and file hash
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &fileLocationsTable,
		IndexName:              aws.String("workspace-file-index"),
		KeyConditionExpression: aws.String("workspace_id = :workspace_id AND file_hash = :file_hash"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
			":file_hash":    &types.AttributeValueMemberS{Value: fileHash},
		},
	})

	if err != nil {
		logger.Error("Failed to query file locations for file hash",
			"error", err,
			"workspace_id", workspaceID,
			"file_hash", fileHash,
		)
		return 0, err
	}

	// Delete each file location
	deleteCount := 0
	for _, item := range result.Items {
		var location WorkspaceFileLocation
		if err := attributevalue.UnmarshalMap(item, &location); err != nil {
			logger.Error("Failed to unmarshal file location for deletion",
				"error", err,
				"workspace_id", workspaceID,
				"file_hash", fileHash,
			)
			continue
		}

		if err := DeleteFileLocation(ctx, location.InstanceID, location.FileHash); err != nil {
			logger.Error("Failed to delete file location",
				"error", err,
				"instance_id", location.InstanceID,
				"file_hash", location.FileHash,
				"workspace_id", workspaceID,
			)
			// Continue with other deletions even if one fails
		} else {
			deleteCount++
		}
	}

	logger.Info("Deleted file locations for file hash",
		"workspace_id", workspaceID,
		"file_hash", fileHash,
		"delete_count", deleteCount,
	)

	return deleteCount, nil
}

// DeleteFileLocationsByWorkspace removes all file locations for a specific workspace
// This is called when a workspace is deleted
func DeleteFileLocationsByWorkspace(ctx context.Context, workspaceID string) error {
	logger.Info("Deleting all file locations for workspace",
		"workspace_id", workspaceID,
	)

	// Query using the GSI to find all file locations for this workspace
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &fileLocationsTable,
		IndexName:              aws.String("workspace-file-index"),
		KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})

	if err != nil {
		logger.Error("Failed to query file locations for workspace",
			"error", err,
			"workspace_id", workspaceID,
		)
		return err
	}

	// Delete each file location
	for _, item := range result.Items {
		var location WorkspaceFileLocation
		if err := attributevalue.UnmarshalMap(item, &location); err != nil {
			logger.Error("Failed to unmarshal file location for deletion",
				"error", err,
				"workspace_id", workspaceID,
			)
			continue
		}

		if err := DeleteFileLocation(ctx, location.InstanceID, location.FileHash); err != nil {
			logger.Error("Failed to delete file location",
				"error", err,
				"instance_id", location.InstanceID,
				"file_hash", location.FileHash,
				"workspace_id", workspaceID,
			)
			// Continue with other deletions even if one fails
		}
	}

	return nil
}

// ValidateInstanceID checks if an instance ID is valid (non-empty UUID format)
func ValidateInstanceID(instanceID string) error {
	if instanceID == "" {
		return fmt.Errorf("instance ID cannot be empty")
	}
	// Basic UUID format validation (36 characters with hyphens)
	if len(instanceID) != 36 {
		return fmt.Errorf("instance ID must be a valid UUID")
	}
	return nil
}

// ValidateAbsoluteFilePath checks if an absolute file path is valid
func ValidateAbsoluteFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	// Ensure it's an absolute path (starts with / on Unix or C:\ on Windows)
	if len(filePath) == 0 {
		return fmt.Errorf("file path cannot be empty")
	}
	// Basic validation - should start with / (Unix) or drive letter (Windows)
	if filePath[0] != '/' && !(len(filePath) >= 3 && filePath[1] == ':' && (filePath[2] == '\\' || filePath[2] == '/')) {
		return fmt.Errorf("file path must be absolute")
	}
	return nil
}
