package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	rateLimitsTable string
)

// RateLimitEntry represents a rate limit record in DynamoDB
type RateLimitEntry struct {
	LicenseKeyHash string `dynamodbav:"license_key_hash"` // Partition key
	Endpoint       string `dynamodbav:"endpoint"`         // Sort key
	RequestCount   int64  `dynamodbav:"request_count"`
	WindowStart    int64  `dynamodbav:"window_start"`
	TTL            int64  `dynamodbav:"ttl"` // Auto-expiration
}

// RateLimitConfig defines rate limits for different license tiers and endpoints
type RateLimitConfig struct {
	EndpointLimits map[string]map[string]int `json:"endpoint_limits"` // license_type -> endpoint -> requests_per_window
	WindowSeconds  int                       `json:"window_seconds"`  // Time window in seconds
}

// Default rate limit configuration
var defaultRateLimitConfig = RateLimitConfig{
	EndpointLimits: map[string]map[string]int{
		// Limits are per minute
		"basic": {
			"workspaces":     10,
			"file-locations": 100,
			"annotations":    1000,
			"auth":           5,
		},
		"premium": { // Just an example, not used atm
			"workspaces":     100,
			"file-locations": 500,
			"annotations":    5000,
			"auth":           10,
		},
	},
	WindowSeconds: 60, // 1-minute windows
}

// Endpoint categorization for rate limiting
var endpointCategories = map[string]string{
	// Workspace endpoints
	"/workspaces":                                  "workspaces",
	"/workspaces/{workspace_id}":                   "workspaces",
	"/workspaces/{workspace_id}/convert-to-shared": "workspaces",
	"/workspaces/{workspace_id}/members":           "workspaces",
	"/workspaces/{workspace_id}/members/{email}":   "workspaces",
	// File endpoints
	"/workspaces/{workspace_id}/files":             "files",
	"/workspaces/{workspace_id}/files/{file_hash}": "files",
	// Annotation endpoints
	"/workspaces/{workspace_id}/annotations":                 "annotations",
	"/workspaces/{workspace_id}/annotations/{annotation_id}": "annotations",
	// Auth endpoints
	"/auth/request-pin": "auth",
	"/auth/verify-pin":  "auth",
	"/auth/refresh":     "auth",
	"/auth/logout":      "auth",
	// File locations
	"/file-locations":     "file-locations",
	"/file-locations/all": "file-locations",
}

// InitRateLimits initializes the rate limiting system
func InitRateLimits(initParams InitParams, rateLimitsTableName string) {
	logger = initParams.Logger
	rateLimitsTable = rateLimitsTableName

	// Initialize the rate limiting system with default config
	logger.Info("Rate limiting initialized", "table", rateLimitsTable)
}

// GetEndpointCategory determines the rate limit category for an API endpoint
func GetEndpointCategory(path string) string {
	for pattern, category := range endpointCategories {
		if path == pattern {
			return category
		}
		// Simple path matching for dynamic endpoints
		if len(pattern) > 0 && pattern[0] == '/' {
			// Convert patterns like "/workspaces/{workspace_id}" to match actual paths
			patternParts := splitPath(pattern)
			pathParts := splitPath(path)

			if len(patternParts) == len(pathParts) {
				match := true
				for i, part := range patternParts {
					if len(part) > 0 && part[0] != '{' && part != pathParts[i] {
						match = false
						break
					}
				}
				if match {
					return category
				}
			}
		}
	}

	// Default category for unknown endpoints
	return "default"
}

// splitPath helper function to split URL paths
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}

	parts := []string{}
	current := ""
	for _, char := range path {
		if char == '/' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}

	return parts
}

// GetLicenseTier determines the license tier based on license data
// For now, we'll implement basic logic - this can be enhanced based on your license structure
func GetLicenseTier(licenseData *LicenseData) string {
	// You can customize this logic based on your license data structure
	// For example, different license IDs or email domains might indicate different tiers
	if licenseData != nil {
		// Add your tier determination logic here
		// For now, default to "basic"
		return "basic"
	}
	return "basic"
}

// CheckRateLimit checks if a request should be allowed based on rate limits
func CheckRateLimit(ctx context.Context, licenseHash, endpoint string, licenseTier string, config RateLimitConfig) (bool, *RateLimitEntry, error) {
	category := GetEndpointCategory(endpoint)

	// Get limits for this license tier and endpoint category
	tierLimits, tierExists := config.EndpointLimits[licenseTier]
	if !tierExists {
		// Default to basic tier limits if tier not found
		tierLimits = config.EndpointLimits["basic"]
	}

	limit, limitExists := tierLimits[category]
	if !limitExists {
		// Default to a conservative limit if category not found
		limit = 10
	}

	now := time.Now().Unix()
	windowStart := now - (now % int64(config.WindowSeconds))
	ttl := windowStart + int64(config.WindowSeconds) + 60 // 60s buffer

	logger.Debug("Checking rate limit",
		"license_hash", licenseHash[:8]+"...",
		"endpoint", endpoint,
		"category", category,
		"tier", licenseTier,
		"limit", limit,
		"window_start", windowStart,
	)

	// First, try to reset the counter if we're in a new time window
	// This handles the case where window_start < current (new window)
	resetInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(rateLimitsTable),
		Key: map[string]types.AttributeValue{
			"license_key_hash": &types.AttributeValueMemberS{Value: licenseHash},
			"endpoint":         &types.AttributeValueMemberS{Value: category},
		},
		UpdateExpression: aws.String("SET request_count = :one, window_start = :start, #ttl = :ttl"),
		// Only reset if: entry exists AND window_start is from a previous window
		ConditionExpression: aws.String("attribute_exists(window_start) AND window_start < :current"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one":     &types.AttributeValueMemberN{Value: "1"},
			":start":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":current": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":ttl":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttl)},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	}

	result, err := ddbClient.UpdateItem(ctx, resetInput)
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if !errors.As(err, &ccfe) {
			// Real error, not just condition failed
			logger.Error("Failed to reset rate limit window", "error", err, "license_hash", licenseHash[:8]+"...")
			return false, nil, fmt.Errorf("failed to reset rate limit window: %w", err)
		}
		// Condition failed means either: entry doesn't exist, or we're in the same window
		// Continue to increment logic below
	} else {
		// Successfully reset the window - this counts as request #1
		var entry RateLimitEntry
		if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
			logger.Error("Failed to unmarshal rate limit entry after reset", "error", err)
			return true, nil, nil
		}
		logger.Debug("Rate limit window reset",
			"license_hash", licenseHash[:8]+"...",
			"endpoint", endpoint,
			"category", category,
			"request_count", entry.RequestCount,
			"limit", limit,
		)
		return true, &entry, nil
	}

	// Either entry doesn't exist or we're in the same window - try to increment
	incrementInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(rateLimitsTable),
		Key: map[string]types.AttributeValue{
			"license_key_hash": &types.AttributeValueMemberS{Value: licenseHash},
			"endpoint":         &types.AttributeValueMemberS{Value: category},
		},
		UpdateExpression: aws.String("SET request_count = if_not_exists(request_count, :zero) + :one, window_start = if_not_exists(window_start, :start), #ttl = :ttl"),
		// Allow if: entry doesn't exist OR (same window AND under limit)
		ConditionExpression: aws.String("attribute_not_exists(request_count) OR (window_start = :current AND request_count < :limit)"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero":    &types.AttributeValueMemberN{Value: "0"},
			":one":     &types.AttributeValueMemberN{Value: "1"},
			":limit":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", limit)},
			":start":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":current": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":ttl":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttl)},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	}

	result, err = ddbClient.UpdateItem(ctx, incrementInput)
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			// Rate limit exceeded - get current count for logging
			currentEntry, getErr := getCurrentRateLimitEntry(ctx, licenseHash, category)
			if getErr == nil && currentEntry != nil {
				logger.Info("Rate limit exceeded",
					"license_hash", licenseHash[:8]+"...",
					"endpoint", endpoint,
					"category", category,
					"current_count", currentEntry.RequestCount,
					"limit", limit,
				)
				return false, currentEntry, nil
			}

			logger.Info("Rate limit exceeded",
				"license_hash", licenseHash[:8]+"...",
				"endpoint", endpoint,
				"category", category,
				"limit", limit,
			)
			return false, nil, nil
		}

		logger.Error("Failed to check rate limit", "error", err, "license_hash", licenseHash[:8]+"...")
		return false, nil, fmt.Errorf("failed to check rate limit: %w", err)
	}

	// Parse the updated entry
	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
		logger.Error("Failed to unmarshal rate limit entry", "error", err)
		return true, nil, nil
	}

	logger.Debug("Rate limit check passed",
		"license_hash", licenseHash[:8]+"...",
		"endpoint", endpoint,
		"category", category,
		"request_count", entry.RequestCount,
		"limit", limit,
	)

	return true, &entry, nil
}

// getCurrentRateLimitEntry retrieves the current rate limit entry
func getCurrentRateLimitEntry(ctx context.Context, licenseHash, category string) (*RateLimitEntry, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(rateLimitsTable),
		Key: map[string]types.AttributeValue{
			"license_key_hash": &types.AttributeValueMemberS{Value: licenseHash},
			"endpoint":         &types.AttributeValueMemberS{Value: category},
		},
	}

	result, err := ddbClient.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, nil
	}

	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Item, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// GetRateLimitStatus returns the current rate limit status for a license
func GetRateLimitStatus(ctx context.Context, licenseHash string) (map[string]*RateLimitEntry, error) {
	// Query all entries for this license hash
	input := &dynamodb.QueryInput{
		TableName:              aws.String(rateLimitsTable),
		KeyConditionExpression: aws.String("license_key_hash = :hash"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":hash": &types.AttributeValueMemberS{Value: licenseHash},
		},
	}

	result, err := ddbClient.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	entries := make(map[string]*RateLimitEntry)
	for _, item := range result.Items {
		var entry RateLimitEntry
		if err := attributevalue.UnmarshalMap(item, &entry); err != nil {
			logger.Error("Failed to unmarshal rate limit entry", "error", err)
			continue
		}
		entries[entry.Endpoint] = &entry
	}

	return entries, nil
}

// CleanupExpiredEntries removes expired rate limit entries (handled by TTL, but for manual cleanup)
func CleanupExpiredEntries(ctx context.Context) error {
	// This is typically handled by DynamoDB TTL, but can be used for manual cleanup
	logger.Info("Manual cleanup of expired rate limit entries is not required when TTL is enabled")
	return nil
}

// CheckRateLimitByIdentifier checks rate limit using a generic identifier (email or license hash)
// This allows rate limiting for both authenticated (license hash) and unauthenticated (email) endpoints.
// The identifier should be prefixed to distinguish types: "email:user@example.com" or "license:sha256:..."
func CheckRateLimitByIdentifier(ctx context.Context, identifier, endpoint string, config RateLimitConfig) (bool, *RateLimitEntry, error) {
	category := GetEndpointCategory(endpoint)

	// Use basic tier limits for auth endpoints
	tierLimits := config.EndpointLimits["basic"]
	limit, limitExists := tierLimits[category]
	if !limitExists {
		limit = 10
	}

	now := time.Now().Unix()
	windowStart := now - (now % int64(config.WindowSeconds))
	ttl := windowStart + int64(config.WindowSeconds) + 60 // 60s buffer

	logger.Debug("Checking rate limit by identifier",
		"identifier_prefix", truncateIdentifier(identifier),
		"endpoint", endpoint,
		"category", category,
		"limit", limit,
		"window_start", windowStart,
	)

	// First, try to reset the counter if we're in a new time window
	resetInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(rateLimitsTable),
		Key: map[string]types.AttributeValue{
			"license_key_hash": &types.AttributeValueMemberS{Value: identifier},
			"endpoint":         &types.AttributeValueMemberS{Value: category},
		},
		UpdateExpression:    aws.String("SET request_count = :one, window_start = :start, #ttl = :ttl"),
		ConditionExpression: aws.String("attribute_exists(window_start) AND window_start < :current"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one":     &types.AttributeValueMemberN{Value: "1"},
			":start":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":current": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":ttl":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttl)},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	}

	result, err := ddbClient.UpdateItem(ctx, resetInput)
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if !errors.As(err, &ccfe) {
			logger.Error("Failed to reset rate limit window", "error", err, "identifier_prefix", truncateIdentifier(identifier))
			return false, nil, fmt.Errorf("failed to reset rate limit window: %w", err)
		}
	} else {
		var entry RateLimitEntry
		if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
			logger.Error("Failed to unmarshal rate limit entry after reset", "error", err)
			return true, nil, nil
		}
		logger.Debug("Rate limit window reset",
			"identifier_prefix", truncateIdentifier(identifier),
			"endpoint", endpoint,
			"category", category,
			"request_count", entry.RequestCount,
			"limit", limit,
		)
		return true, &entry, nil
	}

	// Either entry doesn't exist or we're in the same window - try to increment
	incrementInput := &dynamodb.UpdateItemInput{
		TableName: aws.String(rateLimitsTable),
		Key: map[string]types.AttributeValue{
			"license_key_hash": &types.AttributeValueMemberS{Value: identifier},
			"endpoint":         &types.AttributeValueMemberS{Value: category},
		},
		UpdateExpression:    aws.String("SET request_count = if_not_exists(request_count, :zero) + :one, window_start = if_not_exists(window_start, :start), #ttl = :ttl"),
		ConditionExpression: aws.String("attribute_not_exists(request_count) OR (window_start = :current AND request_count < :limit)"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero":    &types.AttributeValueMemberN{Value: "0"},
			":one":     &types.AttributeValueMemberN{Value: "1"},
			":limit":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", limit)},
			":start":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":current": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowStart)},
			":ttl":     &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttl)},
		},
		ReturnValues: types.ReturnValueUpdatedNew,
	}

	result, err = ddbClient.UpdateItem(ctx, incrementInput)
	if err != nil {
		var ccfe *types.ConditionalCheckFailedException
		if errors.As(err, &ccfe) {
			currentEntry, getErr := getCurrentRateLimitEntry(ctx, identifier, category)
			if getErr == nil && currentEntry != nil {
				logger.Info("Rate limit exceeded",
					"identifier_prefix", truncateIdentifier(identifier),
					"endpoint", endpoint,
					"category", category,
					"current_count", currentEntry.RequestCount,
					"limit", limit,
				)
				return false, currentEntry, nil
			}

			logger.Info("Rate limit exceeded",
				"identifier_prefix", truncateIdentifier(identifier),
				"endpoint", endpoint,
				"category", category,
				"limit", limit,
			)
			return false, nil, nil
		}

		logger.Error("Failed to check rate limit", "error", err, "identifier_prefix", truncateIdentifier(identifier))
		return false, nil, fmt.Errorf("failed to check rate limit: %w", err)
	}

	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
		logger.Error("Failed to unmarshal rate limit entry", "error", err)
		return true, nil, nil
	}

	logger.Debug("Rate limit check passed",
		"identifier_prefix", truncateIdentifier(identifier),
		"endpoint", endpoint,
		"category", category,
		"request_count", entry.RequestCount,
		"limit", limit,
	)

	return true, &entry, nil
}

// truncateIdentifier returns a truncated version of the identifier for logging
func truncateIdentifier(identifier string) string {
	if len(identifier) <= 20 {
		return identifier
	}
	return identifier[:20] + "..."
}

// BuildEmailIdentifier creates a rate limit identifier from an email address
func BuildEmailIdentifier(email string) string {
	return "email:" + email
}

// GetDefaultRateLimitConfig returns the default rate limit configuration
func GetDefaultRateLimitConfig() RateLimitConfig {
	return defaultRateLimitConfig
}
