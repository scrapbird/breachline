package api

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type AddMemberRequest struct {
	WorkspaceID string `json:"workspace_id"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

type AddMemberResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
	Role    string `json:"role"`
}

type RemoveMemberResponse struct {
	Message string `json:"message"`
}

type ListMembersResponse struct {
	Members []Member `json:"members"`
}

type UpdateMemberRequest struct {
	Role string `json:"role"`
}

type UpdateMemberResponse struct {
	Message string `json:"message"`
	Email   string `json:"email"`
	Role    string `json:"role"`
}

type Member struct {
	WorkspaceID string `json:"workspace_id" dynamodbav:"workspace_id"`
	Email       string `json:"email" dynamodbav:"email"`
	Role        string `json:"role" dynamodbav:"role"`
	AddedAt     string `json:"added_at" dynamodbav:"added_at"`
	LastActive  string `json:"last_active" dynamodbav:"last_active"`
}

func CheckMemberExists(ctx context.Context, workspaceID, email string) (bool, error) {
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

func AddMember(ctx context.Context, member Member) error {
	item, err := attributevalue.MarshalMap(member)
	if err != nil {
		logger.Error("Failed to marshal member", "error", err, "workspace_id", member.WorkspaceID)
		return err
	}

	logger.Info("Adding member", "workspace_id", member.WorkspaceID, "email", member.Email, "role", member.Role)
	_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &membersTable,
		Item:      item,
	})
	if err != nil {
		logger.Error("Failed to add member", "error", err, "workspace_id", member.WorkspaceID)
		return err
	}
	incrementMemberCount(ctx, member.WorkspaceID)

	logger.Info("Member added successfully", "workspace_id", member.WorkspaceID, "email", member.Email, "role", member.Role)
	return nil
}

func GetMembers(ctx context.Context, workspaceID string) ([]Member, error) {
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &membersTable,
		KeyConditionExpression: aws.String("workspace_id = :workspace_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
		},
	})
	if err != nil {
		return nil, err
	}

	var members []Member
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &members); err != nil {
		return nil, err
	}

	return members, nil
}

func RemoveMember(ctx context.Context, workspaceID, email string) error {
	logger.Info("Removing member", "workspace_id", workspaceID, "email", email)
	_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &membersTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
			"email":        &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		logger.Error("Failed to remove member", "error", err, "workspace_id", workspaceID)
		return err
	}
	decrementMemberCount(ctx, workspaceID)

	logger.Info("Member removed successfully", "workspace_id", workspaceID, "email", email)
	return nil
}

func incrementMemberCount(ctx context.Context, workspaceID string) {
	logger.Debug("Decrementing member count", "workspace_id", workspaceID)
	ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        &workspacesTable,
		Key:              map[string]types.AttributeValue{"workspace_id": &types.AttributeValueMemberS{Value: workspaceID}},
		UpdateExpression: aws.String("SET member_count = member_count + :inc, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inc":        &types.AttributeValueMemberN{Value: "1"},
			":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
}

func decrementMemberCount(ctx context.Context, workspaceID string) {
	ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        &workspacesTable,
		Key:              map[string]types.AttributeValue{"workspace_id": &types.AttributeValueMemberS{Value: workspaceID}},
		UpdateExpression: aws.String("SET member_count = member_count - :dec, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":dec":        &types.AttributeValueMemberN{Value: "1"},
			":updated_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
}

func UpdateMemberRole(ctx context.Context, workspaceID, email, role string) error {
	_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &membersTable,
		Key: map[string]types.AttributeValue{
			"workspace_id": &types.AttributeValueMemberS{Value: workspaceID},
			"email":        &types.AttributeValueMemberS{Value: email},
		},
		UpdateExpression: aws.String("SET #role = :role"),
		ExpressionAttributeNames: map[string]string{
			"#role": "role",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":role": &types.AttributeValueMemberS{Value: role},
		},
	})

	return err
}
