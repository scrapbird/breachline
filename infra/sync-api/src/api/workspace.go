package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// WorkspaceConfig represents the structure of a workspace config file
type WorkspaceConfig struct {
	HashKey string          `yaml:"hash_key"` // Base64-encoded 32-byte key for HighwayHash
	Files   []WorkspaceFile `yaml:"files"`
}

type WorkspaceMember struct {
	WorkspaceID string `dynamodbav:"workspace_id"`
	Email       string `dynamodbav:"email"`
	Role        string `dynamodbav:"role"`
}

type Workspace struct {
	WorkspaceID string `dynamodbav:"workspace_id"`
	HashKey     string `dynamodbav:"hash_key"` // Base64-encoded 32-byte key for HighwayHash
	Name        string `dynamodbav:"name"`
	OwnerEmail  string `dynamodbav:"owner_email"`
	IsShared    bool   `dynamodbav:"is_shared"`
	MemberCount int    `dynamodbav:"member_count"`
	Version     int64  `dynamodbav:"version"`
	CreatedAt   string `dynamodbav:"created_at"`
	UpdatedAt   string `dynamodbav:"updated_at"`
}

type CreateWorkspaceRequest struct {
	Name string `json:"name"`
}

type CreateWorkspaceResponse struct {
	WorkspaceID string `json:"workspace_id"`
	HashKey     string `json:"hash_key"`
	Name        string `json:"name"`
	OwnerEmail  string `json:"owner_email"`
	IsShared    bool   `json:"is_shared"`
	Version     int64  `json:"version"`
	CreatedAt   string `json:"created_at"`
}

type GetWorkspaceResponse struct {
	WorkspaceID string `json:"workspace_id"`
	HashKey     string `json:"hash_key"`
	Name        string `json:"name"`
	OwnerEmail  string `json:"owner_email"`
	IsShared    bool   `json:"is_shared"`
	MemberCount int    `json:"member_count"`
	Version     int64  `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type UpdateWorkspaceRequest struct {
	Name string `json:"name"`
}

type UpdateWorkspaceResponse struct {
	WorkspaceID string `json:"workspace_id"`
	Name        string `json:"name"`
}

type DeleteWorkspaceResponse struct {
	Message string `json:"message"`
}

type ListWorkspacesResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

type ConvertToSharedResponse struct {
	WorkspaceID string `json:"workspace_id"`
	IsShared    bool   `json:"is_shared"`
}

// GenerateHashKey creates a random 32-byte key for HighwayHash and returns it as base64
func GenerateHashKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

func CreateWorkspace(ctx context.Context, workspace Workspace) error {
	logger.Info("Creating workspace", "workspace_id", workspace.WorkspaceID, "workspace_name", workspace.Name, "owner_email", workspace.OwnerEmail)

	item, err := attributevalue.MarshalMap(workspace)
	if err != nil {
		logger.Error("Failed to marshal workspace", "error", err, "workspace_id", workspace.WorkspaceID)
		return err
	}

	_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &workspacesTable,
		Item:      item,
	})

	if err != nil {
		logger.Error("Failed to put item in DynamoDB", "error", err, "workspace_id", workspace.WorkspaceID)
	} else {
		logger.Info("Successfully stored workspace in DynamoDB", "workspace_id", workspace.WorkspaceID)
	}

	return err
}

func GetWorkspace(ctx context.Context, workspaceID string) (*Workspace, error) {
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &workspacesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("workspace not found")
	}

	var workspace Workspace
	if err := attributevalue.UnmarshalMap(result.Item, &workspace); err != nil {
		return nil, err
	}

	return &workspace, nil
}

func GetWorkspaceCount(ctx context.Context, ownerEmail string) (int, error) {
	// Query workspaces by owner using GSI
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &workspacesTable,
		IndexName:              aws.String("owner-index"),
		KeyConditionExpression: aws.String("owner_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: ownerEmail},
		},
		Select: types.SelectCount,
	})
	if err != nil {
		return 0, err
	}

	return int(result.Count), nil
}

func GetOwnedWorkspaces(ctx context.Context, email string) ([]Workspace, error) {
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &workspacesTable,
		IndexName:              aws.String("owner-index"),
		KeyConditionExpression: aws.String("owner_email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, err
	}

	var workspaces []Workspace
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &workspaces); err != nil {
		return nil, err
	}

	return workspaces, nil
}

func GetMemberWorkspaces(ctx context.Context, email string) ([]Workspace, error) {
	// Query workspace members by email using GSI
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &membersTable,
		IndexName:              aws.String("user-workspaces-index"),
		KeyConditionExpression: aws.String("email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, err
	}

	var members []WorkspaceMember
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &members); err != nil {
		return nil, err
	}

	// Fetch workspace details for each membership
	workspaces := make([]Workspace, 0, len(members))
	for _, member := range members {
		workspace, err := GetWorkspace(ctx, member.WorkspaceID)
		if err != nil {
			log.Printf("Failed to get workspace %s: %v", member.WorkspaceID, err)
			continue
		}

		workspaces = append(workspaces, *workspace)
	}

	return workspaces, nil
}

func UpdateWorkspace(ctx context.Context, workspaceID string, name string) error {
	// Update workspace
	logger.Info("Updating workspace", "workspace_id", workspaceID, "new_name", name)
	updatedAt := time.Now().Format(time.RFC3339)
	_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &workspacesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
		UpdateExpression: aws.String("SET #name = :name, updated_at = :updated_at"),
		ExpressionAttributeNames: map[string]string{
			"#name": "name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name":       &types.AttributeValueMemberS{Value: name},
			":updated_at": &types.AttributeValueMemberS{Value: updatedAt},
		},
	})
	if err != nil {
		logger.Error("Failed to update workspace", "error", err, "workspace_id", workspaceID)
		return err
	}

	return nil
}

// UpdateWorkspaceVersion is deprecated - use UpdateWorkspaceTimestamp from audit.go instead

func ConvertToShared(ctx context.Context, workspaceID string) error {
	// Convert to shared
	logger.Info("Converting workspace to shared", "workspace_id", workspaceID)
	updatedAt := time.Now().Format(time.RFC3339)
	_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &workspacesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
		UpdateExpression: aws.String("SET is_shared = :is_shared, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":is_shared":  &types.AttributeValueMemberBOOL{Value: true},
			":updated_at": &types.AttributeValueMemberS{Value: updatedAt},
		},
	})

	if err != nil {
		logger.Error("Failed to convert workspace", "error", err, "workspace_id", workspaceID)
		return err
	}

	return nil
}

func CheckWorkspaceAccess(ctx context.Context, workspaceID, email string) (bool, error) {
	// Check if user is owner
	workspace, err := GetWorkspace(ctx, workspaceID)
	if err != nil {
		return false, err
	}

	if workspace.OwnerEmail == email {
		return true, nil
	}

	// Check if user is a member
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &membersTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
			"email":        &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return false, err
	}

	return result.Item != nil, nil
}

func CheckWritePermission(ctx context.Context, workspaceID, email, ownerEmail string) (bool, error) {
	// Owner always has write permission
	if email == ownerEmail {
		return true, nil
	}

	// Check member role
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &membersTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
			"email":        &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return false, err
	}

	if result.Item == nil {
		return false, nil
	}

	role := result.Item["role"].(*types.AttributeValueMemberS).Value
	return role != "viewer", nil
}

func DeleteWorkspaceData(ctx context.Context, workspaceID string) error {
	// Delete members
	if err := deleteMembers(ctx, workspaceID); err != nil {
		return err
	}

	// Delete annotations
	if err := deleteAnnotations(ctx, workspaceID); err != nil {
		return err
	}

	// Delete files
	if err := deleteFiles(ctx, workspaceID); err != nil {
		return err
	}

	// Delete workspace
	_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &workspacesTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})

	return err
}

func deleteMembers(ctx context.Context, workspaceID string) error {
	// Query all members
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &membersTable,
		KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})
	if err != nil {
		return err
	}

	// Delete each member
	for _, item := range result.Items {
		email := item["email"].(*types.AttributeValueMemberS).Value
		_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: &membersTable,
			Key: map[string]types.AttributeValue{
				"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
				"email":        &types.AttributeValueMemberS{Value: email},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteAnnotations(ctx context.Context, workspaceID string) error {
	// Query all annotations
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &annotationsTable,
		KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})
	if err != nil {
		return err
	}

	// Delete each annotation
	for _, item := range result.Items {
		annotationID := item["annotation_id"].(*types.AttributeValueMemberS).Value
		_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: &annotationsTable,
			Key: map[string]types.AttributeValue{
				"workspace_id":  &types.AttributeValueMemberS{Value: workspaceID},
				"annotation_id": &types.AttributeValueMemberS{Value: annotationID},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteFiles(ctx context.Context, workspaceID string) error {
	// Query all files
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &filesTable,
		KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})
	if err != nil {
		return err
	}

	// Delete each file
	for _, item := range result.Items {
		filePath := item["file_path"].(*types.AttributeValueMemberS).Value
		_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: &filesTable,
			Key: map[string]types.AttributeValue{
				"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
				"file_path":    &types.AttributeValueMemberS{Value: filePath},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// deleteChanges is deprecated - changes table no longer exists

func GenerateWorkspaceID() string {
	return fmt.Sprintf("ws_%s", uuid.New().String())
}
