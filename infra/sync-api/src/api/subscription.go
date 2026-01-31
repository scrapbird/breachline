package api

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type UserSubscription struct {
	Email          string `dynamodbav:"email"`
	WorkspaceLimit int    `dynamodbav:"workspace_limit"`
	SeatCount      int    `dynamodbav:"seat_count"`
	CreatedAt      string `dynamodbav:"created_at"`
	UpdatedAt      string `dynamodbav:"updated_at"`
}

func GetUserSubscription(ctx context.Context, email string) (*UserSubscription, error) {
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &subscriptionsTable,
		Key: map[string]types.AttributeValue{
			"email": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("subscription not found")
	}

	var subscription UserSubscription
	if err := attributevalue.UnmarshalMap(result.Item, &subscription); err != nil {
		return nil, err
	}

	return &subscription, nil
}
