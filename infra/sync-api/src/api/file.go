package api

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	sharedtypes "github.com/scrapbird/breachline/shared/types"
)

// FileOptions is an alias to the shared type for API compatibility.
// Use sharedtypes.FileOptions directly for new code.
type FileOptions = sharedtypes.FileOptions

// WorkspaceFile represents workspace information for a single file
// This struct serves dual purpose: YAML serialization for config files
// and DynamoDB storage for file metadata
type WorkspaceFile struct {
	WorkspaceID    string       `json:"workspace_id,omitempty" yaml:"-" dynamodbav:"workspace_id"`
	FileIdentifier string       `json:"file_identifier,omitempty" yaml:"-" dynamodbav:"file_identifier"`             // Composite key: {file_hash}#{options.Key()}
	FileHash       string       `json:"file_hash" yaml:"hash,omitempty" dynamodbav:"file_hash"`                      // HighwayHash of the file content
	Options        FileOptions  `json:"options" yaml:"options" dynamodbav:"options"`                                 // File options (jpath, noHeaderRow, ingestTimezoneOverride)
	Description    string       `json:"description,omitempty" yaml:"description,omitempty" dynamodbav:"description"` // User-provided description
	Version        int64        `json:"version,omitempty" yaml:"-" dynamodbav:"version"`
	CreatedAt      string       `json:"created_at,omitempty" yaml:"-" dynamodbav:"created_at"`
	UpdatedAt      string       `json:"updated_at,omitempty" yaml:"-" dynamodbav:"updated_at"`
	CreatedBy      string       `json:"created_by,omitempty" yaml:"-" dynamodbav:"created_by"`
	Annotations    []Annotation `json:"annotations,omitempty" yaml:"annotations,omitempty" dynamodbav:"-"` // For YAML compatibility, not stored in files table
}

// MakeFileIdentifier creates a composite file identifier from file hash and options
// Format: {file_hash}#{options.Key()}
func MakeFileIdentifier(fileHash string, opts FileOptions) string {
	return fileHash + "#" + opts.Key()
}

type CreateFileRequest struct {
	WorkspaceID string      `json:"workspace_id"`
	FileHash    string      `json:"file_hash"`
	Options     FileOptions `json:"options"` // File options (jpath, noHeaderRow, ingestTimezoneOverride)
	Description string      `json:"description,omitempty"`
	FilePath    string      `json:"file_path,omitempty"`   // Local file path on the client instance
	InstanceID  string      `json:"instance_id,omitempty"` // Client instance ID for file location tracking
}

type UpdateFileRequest struct {
	FileHash    string      `json:"file_hash"`
	Options     FileOptions `json:"options"` // File options (jpath, noHeaderRow, ingestTimezoneOverride)
	Description string      `json:"description,omitempty"`
}

type DeleteFileRequest struct {
	FileHash string      `json:"file_hash"`
	Options  FileOptions `json:"options"` // File options (jpath, noHeaderRow, ingestTimezoneOverride)
}

type ListFilesResponse struct {
	Files []WorkspaceFile `json:"files"`
}

type FileResponse struct {
	Message     string `json:"message"`
	WorkspaceID string `json:"workspace_id"`
	FileHash    string `json:"file_hash"`
}

// CreateFile inserts a new file record into DynamoDB
func CreateFile(ctx context.Context, file WorkspaceFile) error {
	// Ensure file_identifier is set
	if file.FileIdentifier == "" {
		file.FileIdentifier = MakeFileIdentifier(file.FileHash, file.Options)
	}

	logger.Info("Creating file",
		"workspace_id", file.WorkspaceID,
		"file_hash", file.FileHash,
		"file_identifier", file.FileIdentifier,
	)

	item, err := attributevalue.MarshalMap(file)
	if err != nil {
		logger.Error("Failed to marshal file",
			"error", err,
			"file_hash", file.FileHash,
			"workspace_id", file.WorkspaceID,
		)
		return err
	}

	logger.Debug("Putting file item to DynamoDB",
		"file_hash", file.FileHash,
		"table", filesTable,
	)

	_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &filesTable,
		Item:      item,
	})

	if err != nil {
		logger.Error("Failed to create file item",
			"error", err,
			"file_hash", file.FileHash,
			"table", filesTable,
		)
		return err
	}

	return nil
}

// UpdateFile updates an existing file record in DynamoDB by file identifier
// Note: jpath and noHeaderRow are part of the key and cannot be changed; only description can be updated
func UpdateFile(ctx context.Context, workspaceID string, fileIdentifier string, description string, version int64) error {
	logger.Info("Updating file",
		"workspace_id", workspaceID,
		"file_identifier", fileIdentifier,
		"description", description,
	)

	// Build update expression dynamically based on what fields are provided
	updateExpr := "SET updated_at = :updated_at, version = :version"
	exprValues := map[string]types.AttributeValue{
		":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		":version":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", version)},
	}

	if description != "" {
		updateExpr += ", description = :description"
		exprValues[":description"] = &types.AttributeValueMemberS{Value: description}
	}

	logger.Debug("Executing DynamoDB update",
		"file_identifier", fileIdentifier,
		"workspace_id", workspaceID,
		"table", filesTable,
	)

	_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &filesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id":    &types.AttributeValueMemberS{Value: workspaceID},
			"file_identifier": &types.AttributeValueMemberS{Value: fileIdentifier},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprValues,
	})
	if err != nil {
		logger.Error("Failed to update file",
			"error", err,
			"file_identifier", fileIdentifier,
			"workspace_id", workspaceID,
			"table", filesTable,
		)
		return err
	}

	return nil
}

// GetFile retrieves a single file record from DynamoDB by file identifier
func GetFile(ctx context.Context, workspaceID string, fileIdentifier string) (*WorkspaceFile, error) {
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &filesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id":    &types.AttributeValueMemberS{Value: workspaceID},
			"file_identifier": &types.AttributeValueMemberS{Value: fileIdentifier},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("file not found")
	}

	var file WorkspaceFile
	if err := attributevalue.UnmarshalMap(result.Item, &file); err != nil {
		return nil, err
	}

	return &file, nil
}

// GetFileByHash retrieves a single file record from DynamoDB by file hash using the GSI
// This is useful for backward compatibility or when jpath/noHeaderRow are not known
func GetFileByHash(ctx context.Context, workspaceID string, fileHash string) (*WorkspaceFile, error) {
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &filesTable,
		IndexName:              aws.String("file-hash-index"),
		KeyConditionExpression: aws.String("workspace_id = :workspace_id AND file_hash = :file_hash"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
			":file_hash":    &types.AttributeValueMemberS{Value: fileHash},
		},
		Limit: aws.Int32(1),
	})
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("file not found")
	}

	var file WorkspaceFile
	if err := attributevalue.UnmarshalMap(result.Items[0], &file); err != nil {
		return nil, err
	}

	return &file, nil
}

// ListFiles retrieves all file records for a workspace from DynamoDB
func ListFiles(ctx context.Context, workspaceID string) ([]WorkspaceFile, error) {
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &filesTable,
		KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})

	if err != nil {
		return nil, err
	}

	var files []WorkspaceFile
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &files); err != nil {
		return nil, err
	}

	return files, nil
}

// DeleteFile removes a file record from DynamoDB by file identifier
func DeleteFile(ctx context.Context, workspaceID string, fileIdentifier string) error {
	logger.Debug("Deleting file",
		"file_identifier", fileIdentifier,
		"workspace_id", workspaceID,
	)

	_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &filesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id":    &types.AttributeValueMemberS{Value: workspaceID},
			"file_identifier": &types.AttributeValueMemberS{Value: fileIdentifier},
		},
	})
	if err != nil {
		logger.Error("Failed to delete file",
			"error", err,
			"file_identifier", fileIdentifier,
			"workspace_id", workspaceID,
		)
		return err
	}

	return nil
}

// DeleteFileByHash removes all file records from DynamoDB matching the file hash
// This is useful for removing all variants of a file (different jpath/noHeaderRow combinations)
func DeleteFileByHash(ctx context.Context, workspaceID string, fileHash string) error {
	logger.Debug("Deleting all files by hash",
		"file_hash", fileHash,
		"workspace_id", workspaceID,
	)

	// Get all files for the workspace and filter by hash client-side
	// This is more reliable than using a GSI which may have replication lag
	files, err := ListFiles(ctx, workspaceID)
	if err != nil {
		logger.Error("Failed to list files for deletion",
			"error", err,
			"workspace_id", workspaceID,
		)
		return err
	}

	// Filter and delete files matching the hash
	deleteCount := 0
	for _, file := range files {
		if file.FileHash == fileHash {
			logger.Debug("Deleting file variant",
				"file_identifier", file.FileIdentifier,
				"file_hash", fileHash,
			)
			if err := DeleteFile(ctx, workspaceID, file.FileIdentifier); err != nil {
				logger.Error("Failed to delete file variant",
					"error", err,
					"file_identifier", file.FileIdentifier,
				)
			} else {
				deleteCount++
			}
		}
	}

	logger.Info("Deleted files by hash",
		"file_hash", fileHash,
		"workspace_id", workspaceID,
		"delete_count", deleteCount,
	)

	return nil
}

// ValidateFilePath checks if a file path is valid
func ValidateFilePath(filePath string) error {
	if filePath == "" {
		return fmt.Errorf("file path cannot be empty")
	}
	return nil
}

// ValidateFileHash checks if a file hash is valid (non-empty)
func ValidateFileHash(fileHash string) error {
	if fileHash == "" {
		return fmt.Errorf("file hash cannot be empty")
	}
	return nil
}
