package api

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type RequestPINRequest struct {
	License string `json:"license"`
}

type RequestPINResponse struct {
	Message   string `json:"message"`
	ExpiresAt string `json:"expires_at"`
	PinID     string `json:"pin_id"`
}

type VerifyPINRequest struct {
	License string `json:"license"`
	PIN     string `json:"pin"`
}

type VerifyPINResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type PIN struct {
	Email          string `dynamodbav:"email"`
	PinHash        string `dynamodbav:"pin_hash"`
	LicenseKeyHash string `dynamodbav:"license_key_hash"`
	ExpiresAt      int64  `dynamodbav:"expires_at"`
	CreatedAt      string `dynamodbav:"created_at"`
}

func CreatePin(ctx context.Context, pin PIN) error {
	logger.Debug("Storing PIN in DynamoDB", "user_email", pin.Email, "pin_hash_prefix", pin.PinHash[:8]+"...")
	_, err := ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &pinsTable,
		Item: map[string]types.AttributeValue{
			"email":            &types.AttributeValueMemberS{Value: pin.Email},
			"pin_hash":         &types.AttributeValueMemberS{Value: string(pin.PinHash)},
			"license_key_hash": &types.AttributeValueMemberS{Value: pin.LicenseKeyHash},
			"expires_at":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", pin.ExpiresAt)},
			"created_at":       &types.AttributeValueMemberS{Value: pin.CreatedAt},
		},
	})

	return err
}

// GetValidPINs retrieves all valid (non-expired) PINs for an email
func GetValidPINs(ctx context.Context, email string) ([]PIN, error) {
	logger.Debug("Querying all PINs for email", "user_email", email)

	// Query all PINs for this email
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &pinsTable,
		KeyConditionExpression: &[]string{"email = :email"}[0],
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no PINs found for email")
	}

	var pins []PIN
	currentTime := time.Now().Unix()

	for _, item := range result.Items {
		var pin PIN
		if err := attributevalue.UnmarshalMap(item, &pin); err != nil {
			continue // Skip invalid records
		}

		// Only include non-expired PINs
		if pin.ExpiresAt > currentTime {
			pins = append(pins, pin)
		}
	}

	if len(pins) == 0 {
		return nil, fmt.Errorf("no valid PINs found")
	}

	logger.Debug("Found valid PINs", "user_email", email, "count", len(pins))
	return pins, nil
}

// DeletePIN deletes a specific PIN by email and pin_hash
func DeletePIN(ctx context.Context, email, pinHash string) error {
	logger.Debug("Deleting specific PIN", "user_email", email, "pin_hash_prefix", pinHash[:8]+"...")
	_, err := ddbClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: &pinsTable,
		Key: map[string]types.AttributeValue{
			"email":    &types.AttributeValueMemberS{Value: email},
			"pin_hash": &types.AttributeValueMemberS{Value: pinHash},
		},
	})
	return err
}

// DeleteAllPINs deletes all PINs for an email (for cleanup purposes)
func DeleteAllPINs(ctx context.Context, email string) error {
	logger.Debug("Deleting all PINs for email", "user_email", email)

	// First get all PINs for this email
	result, err := ddbClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              &pinsTable,
		KeyConditionExpression: &[]string{"email = :email"}[0],
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return err
	}

	// Delete each PIN individually
	for _, item := range result.Items {
		var pin PIN
		if err := attributevalue.UnmarshalMap(item, &pin); err != nil {
			continue
		}

		if err := DeletePIN(ctx, email, pin.PinHash); err != nil {
			logger.Error("Failed to delete PIN", "error", err, "user_email", email, "pin_hash_prefix", pin.PinHash[:8]+"...")
		}
	}

	return nil
}

func EnsureUserSubscription(ctx context.Context, licenseData *LicenseData) error {
	// Check if subscription exists
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &subscriptionsTable,
		Key: map[string]types.AttributeValue{
			"email": &types.AttributeValueMemberS{Value: licenseData.Email},
		},
	})
	if err != nil {
		return err
	}

	// If subscription doesn't exist, create it with default values
	if result.Item == nil {
		now := time.Now().Format(time.RFC3339)
		subscription := UserSubscription{
			Email:          licenseData.Email,
			WorkspaceLimit: 0, // Default: 0 workspaces (must purchase subscription)
			SeatCount:      0, // Default: 0 seats (must purchase subscription)
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		item, err := attributevalue.MarshalMap(subscription)
		if err != nil {
			return err
		}

		_, err = ddbClient.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: &subscriptionsTable,
			Item:      item,
		})
		return err
	}

	return nil
}
