package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-lambda-go/events"
)

// RateLimitMiddleware provides rate limiting functionality for API endpoints
type RateLimitMiddleware struct {
	rateLimitConfig RateLimitConfig
	logger          *slog.Logger
}

// NewRateLimitMiddleware creates a new rate limiting middleware instance
func NewRateLimitMiddleware(config RateLimitConfig, logger *slog.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		rateLimitConfig: config,
		logger:          logger,
	}
}

// CheckRateLimit checks if the request should be allowed based on rate limits
// Returns (allowed, rateLimitHeaders, rateLimitInfo, error)
func (m *RateLimitMiddleware) CheckRateLimit(ctx context.Context, request events.APIGatewayProxyRequest) (bool, map[string]string, *RateLimitInfo, error) {
	// Extract license key hash from authorizer context
	licenseKeyHash, ok := request.RequestContext.Authorizer["license_key_hash"].(string)
	if !ok || licenseKeyHash == "" {
		m.logger.Error("License key hash not found in authorizer context")
		return false, map[string]string{
			"X-RateLimit-Limit":     "0",
			"X-RateLimit-Remaining": "0",
			"X-RateLimit-Reset":     "0",
		}, &RateLimitInfo{Category: "unknown", Limit: 0, ResetInSecs: 0}, nil
	}

	// Determine license tier (for now, default to basic - you can enhance this)
	licenseTier := "basic" // This could be determined from license data if available

	// Get limits for headers (do this early so we have category info)
	category := GetEndpointCategory(request.Path)
	tierLimits := m.rateLimitConfig.EndpointLimits[licenseTier]
	limit, exists := tierLimits[category]
	if !exists {
		limit = 10 // Default limit
	}

	// Calculate reset time (next window start)
	now := time.Now().Unix()
	windowStart := now - (now % int64(m.rateLimitConfig.WindowSeconds))
	resetTime := windowStart + int64(m.rateLimitConfig.WindowSeconds)
	resetInSecs := resetTime - now

	// Build rate limit info for error responses
	rateLimitInfo := &RateLimitInfo{
		Category:    category,
		Limit:       limit,
		ResetInSecs: resetInSecs,
	}

	// Check rate limit using the API function
	allowed, entry, err := CheckRateLimit(
		ctx,
		licenseKeyHash,
		request.Path,
		licenseTier,
		m.rateLimitConfig,
	)

	if err != nil {
		m.logger.Error("Rate limit check failed", "error", err, "license_hash", licenseKeyHash[:8]+"...")
		return false, nil, rateLimitInfo, err
	}

	remaining := int64(0)
	if allowed && entry != nil {
		remaining = int64(limit) - entry.RequestCount
		if remaining < 0 {
			remaining = 0
		}
	}

	headers := map[string]string{
		"X-RateLimit-Limit":     fmt.Sprintf("%d", limit),
		"X-RateLimit-Remaining": fmt.Sprintf("%d", remaining),
		"X-RateLimit-Reset":     fmt.Sprintf("%d", resetTime),
	}

	m.logger.Debug("Rate limit check completed",
		"license_hash", licenseKeyHash[:8]+"...",
		"endpoint", request.Path,
		"category", category,
		"tier", licenseTier,
		"allowed", allowed,
		"remaining", remaining,
		"limit", limit,
	)

	return allowed, headers, rateLimitInfo, nil
}

// RateLimitInfo contains details about the rate limit that was exceeded
type RateLimitInfo struct {
	Category    string
	Limit       int
	ResetInSecs int64
}

// CreateRateLimitErrorResponse creates a rate limit exceeded response with specific limit details
func (m *RateLimitMiddleware) CreateRateLimitErrorResponse(headers map[string]string, info *RateLimitInfo) events.APIGatewayProxyResponse {
	// Copy rate limit headers to response
	responseHeaders := make(map[string]string)
	for k, v := range headers {
		responseHeaders[k] = v
	}
	responseHeaders["Content-Type"] = "application/json"
	responseHeaders["Retry-After"] = fmt.Sprintf("%d", m.rateLimitConfig.WindowSeconds)

	// Build descriptive message with specific limit information
	message := "Rate limit exceeded. Please try again later."
	if info != nil {
		categoryLabel := getCategoryLabel(info.Category)
		message = fmt.Sprintf("Rate limit exceeded for %s operations. Limit: %d requests per minute. Please wait %d seconds before retrying.",
			categoryLabel, info.Limit, info.ResetInSecs)
	}

	// Use the same pattern as CreateErrorResponse in common.go
	// to avoid any issues with inline struct initialization
	errResp := ErrorResponse{}
	errResp.Error.Code = "rate_limit_exceeded"
	errResp.Error.Message = message

	body, _ := json.Marshal(errResp)

	return events.APIGatewayProxyResponse{
		StatusCode: 429,
		Headers:    responseHeaders,
		Body:       string(body),
	}
}

// getCategoryLabel returns a user-friendly label for a rate limit category
func getCategoryLabel(category string) string {
	labels := map[string]string{
		"workspaces":  "workspace",
		"files":       "file",
		"annotations": "annotation",
		"auth":        "authentication",
		"default":     "API",
	}
	if label, ok := labels[category]; ok {
		return label
	}
	return category
}

// RateLimitedHandler wraps a lambda handler with rate limiting
func RateLimitedHandler(handler func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error), logger *slog.Logger) func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	middleware := NewRateLimitMiddleware(defaultRateLimitConfig, logger)

	return func(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		// Check rate limit
		allowed, headers, rateLimitInfo, err := middleware.CheckRateLimit(ctx, request)
		if err != nil {
			middleware.logger.Error("Rate limiting error", "error", err)
			// If rate limiting fails, allow the request but log the error
			// Still call handler but add error indicator headers
			response, handlerErr := handler(ctx, request)
			if response.Headers == nil {
				response.Headers = make(map[string]string)
			}
			response.Headers["X-RateLimit-Error"] = "rate_limit_check_failed"
			return response, handlerErr
		}

		if !allowed {
			middleware.logger.Warn("Request blocked by rate limit",
				"path", request.Path,
				"method", request.HTTPMethod,
				"category", rateLimitInfo.Category,
				"limit", rateLimitInfo.Limit,
				"headers", headers,
			)
			return middleware.CreateRateLimitErrorResponse(headers, rateLimitInfo), nil
		}

		// Call the original handler
		response, err := handler(ctx, request)
		if err != nil {
			return response, err
		}

		// Add rate limit headers to successful responses
		if response.Headers == nil {
			response.Headers = make(map[string]string)
		}
		for k, v := range headers {
			response.Headers[k] = v
		}

		return response, nil
	}
}

// ApplyRateLimitingToHandler is a convenience function to apply rate limiting to existing handlers
func ApplyRateLimitingToHandler(originalHandler func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error), logger *slog.Logger) func(context.Context, events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return RateLimitedHandler(originalHandler, logger)
}

// CheckAuthRateLimit checks rate limit for authentication endpoints using email as the identifier.
// This is used for unauthenticated endpoints like request-pin and verify-pin where we don't have a JWT.
// Returns (allowed, rateLimitHeaders, rateLimitInfo, error)
func (m *RateLimitMiddleware) CheckAuthRateLimit(ctx context.Context, email string, endpoint string) (bool, map[string]string, *RateLimitInfo, error) {
	// Build email-based identifier
	identifier := BuildEmailIdentifier(email)

	// Get limits for headers
	category := GetEndpointCategory(endpoint)
	tierLimits := m.rateLimitConfig.EndpointLimits["basic"]
	limit, exists := tierLimits[category]
	if !exists {
		limit = 5 // Default auth limit
	}

	// Calculate reset time
	now := time.Now().Unix()
	windowStart := now - (now % int64(m.rateLimitConfig.WindowSeconds))
	resetTime := windowStart + int64(m.rateLimitConfig.WindowSeconds)
	resetInSecs := resetTime - now

	// Build rate limit info for error responses
	rateLimitInfo := &RateLimitInfo{
		Category:    category,
		Limit:       limit,
		ResetInSecs: resetInSecs,
	}

	// Check rate limit using the identifier-based function
	allowed, entry, err := CheckRateLimitByIdentifier(
		ctx,
		identifier,
		endpoint,
		m.rateLimitConfig,
	)

	if err != nil {
		m.logger.Error("Auth rate limit check failed", "error", err, "email", email)
		return false, nil, rateLimitInfo, err
	}

	remaining := int64(0)
	if allowed && entry != nil {
		remaining = int64(limit) - entry.RequestCount
		if remaining < 0 {
			remaining = 0
		}
	}

	headers := map[string]string{
		"X-RateLimit-Limit":     fmt.Sprintf("%d", limit),
		"X-RateLimit-Remaining": fmt.Sprintf("%d", remaining),
		"X-RateLimit-Reset":     fmt.Sprintf("%d", resetTime),
	}

	m.logger.Debug("Auth rate limit check completed",
		"email", email,
		"endpoint", endpoint,
		"category", category,
		"allowed", allowed,
		"remaining", remaining,
		"limit", limit,
	)

	return allowed, headers, rateLimitInfo, nil
}

// NewRateLimitMiddlewareWithConfig creates a new rate limiting middleware with custom config
func NewRateLimitMiddlewareWithConfig(config RateLimitConfig, logger *slog.Logger) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		rateLimitConfig: config,
		logger:          logger,
	}
}

// GetDefaultConfig returns the default rate limit configuration for use in lambdas
func GetDefaultConfig() RateLimitConfig {
	return defaultRateLimitConfig
}
