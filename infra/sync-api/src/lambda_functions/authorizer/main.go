package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/scrapbird/breachline/infra/sync-api/src/api"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/golang-jwt/jwt/v5"
)

var (
	smClient         *secretsmanager.Client
	jwtPublicKeyName string

	logger *slog.Logger
)

func init() {
	// Initialize JSON logger for CloudWatch
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("Failed to load AWS config: %v", err)
	}

	smClient = secretsmanager.NewFromConfig(cfg)
	jwtPublicKeyName = os.Getenv("JWT_PUBLIC_KEY")

	// Init API layer
	api.Init(api.InitParams{
		Logger: logger,
	})
}

func handler(ctx context.Context, request events.APIGatewayCustomAuthorizerRequest) (events.APIGatewayCustomAuthorizerResponse, error) {
	logger.Info("Authorization request received", "method_arn", request.MethodArn)

	// Extract token from Authorization header
	token := request.AuthorizationToken
	if token == "" {
		logger.Error("No authorization token provided")
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	// Remove "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")
	logger.Debug("Token extracted from authorization header")

	// Get JWT public key
	logger.Debug("Fetching JWT public key from Secrets Manager")
	result, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: &jwtPublicKeyName,
	})
	if err != nil {
		logger.Error("Failed to get JWT public key", "error", err)
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	// Parse ECDSA public key
	logger.Debug("Parsing JWT public key")
	jwtPubKey, err := api.ParsePublicKey(*result.SecretString)
	if err != nil {
		logger.Error("Failed to parse JWT public key", "error", err)
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	// Parse and validate token
	logger.Debug("Parsing and validating access token")
	var claims api.AccessTokenClaims
	parsedToken, err := jwt.ParseWithClaims(token, &claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtPubKey, nil
	})

	if err != nil || !parsedToken.Valid {
		logger.Error("Token validation failed", "error", err)
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	logger.Info("Token validated", "user_email", claims.Email)

	// Verify token type
	if claims.Type != "access" {
		logger.Error("Invalid token type", "type", claims.Type, "expected", "access", "user_email", claims.Email)
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	// Verify license hasn't expired
	logger.Debug("Checking license expiration", "user_email", claims.Email, "license_expires_at", claims.LicenseExpiresAt)
	licenseExpiresAt, err := time.Parse(time.RFC3339, claims.LicenseExpiresAt)
	if err != nil || time.Now().After(licenseExpiresAt) {
		logger.Error("License expired", "user_email", claims.Email, "license_expires_at", claims.LicenseExpiresAt, "parse_error", err)
		return generatePolicy("", "Deny", request.MethodArn), nil
	}

	logger.Info("Authorization successful", "user_email", claims.Email, "method_arn", request.MethodArn)

	// Generate allow policy with context
	policy := generatePolicy(claims.Email, "Allow", request.MethodArn)
	policy.Context = map[string]interface{}{
		"email":            claims.Email,
		"license_key_hash": claims.LicenseKeyHash,
	}

	return policy, nil
}

func generatePolicy(principalID, effect, resource string) events.APIGatewayCustomAuthorizerResponse {
	authResponse := events.APIGatewayCustomAuthorizerResponse{PrincipalID: principalID}

	if effect != "" && resource != "" {
		// Extract the API Gateway ARN base and allow all methods/resources
		// Format: arn:aws:execute-api:region:account-id:api-id/stage/METHOD/resource
		// We want: arn:aws:execute-api:region:account-id:api-id/stage/*/*
		// Split on the first slash to separate ARN base from path
		arnParts := strings.SplitN(resource, "/", 2)
		var wildcardResource string
		if len(arnParts) >= 2 {
			// arnParts[0] = arn:aws:execute-api:region:account-id:api-id
			// arnParts[1] = stage/METHOD/resource
			stageParts := strings.SplitN(arnParts[1], "/", 2)
			if len(stageParts) >= 1 {
				// Reconstruct with wildcards: arn-base/stage/*/*
				wildcardResource = arnParts[0] + "/" + stageParts[0] + "/*/*"
			} else {
				wildcardResource = resource
			}
		} else {
			wildcardResource = resource
		}

		authResponse.PolicyDocument = events.APIGatewayCustomAuthorizerPolicy{
			Version: "2012-10-17",
			Statement: []events.IAMPolicyStatement{
				{
					Action:   []string{"execute-api:Invoke"},
					Effect:   effect,
					Resource: []string{wildcardResource},
				},
			},
		}
	}

	return authResponse
}

func main() {
	lambda.Start(handler)
}
