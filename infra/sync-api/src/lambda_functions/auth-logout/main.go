package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type LogoutResponse struct {
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

var (
	logger          *slog.Logger
	rateLimitsTable string
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Init API layer
	api.Init(api.InitParams{
		Logger: logger,
	})

	// Init rate limiting
	rateLimitsTable = os.Getenv("RATE_LIMITS_TABLE")
	if rateLimitsTable != "" {
		api.InitRateLimits(api.InitParams{Logger: logger}, rateLimitsTable)
	}
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Apply rate limiting
	rateLimitedHandler := api.ApplyRateLimitingToHandler(authLogoutHandler, logger)
	return rateLimitedHandler(ctx, request)
}

func authLogoutHandler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	email := request.RequestContext.Authorizer["email"].(string)

	logger.Info("Logout request received", "user_email", email)

	// Since tokens are stateless JWT tokens, logout is handled client-side
	// The client should delete the tokens from local storage
	// This endpoint exists for API completeness and future extensibility

	response := LogoutResponse{
		Message: "Logout successful. Please delete tokens from client storage.",
	}

	logger.Info("Logout successful", "user_email", email)

	body, _ := json.Marshal(response)
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}

func main() {
	lambda.Start(handler)
}
