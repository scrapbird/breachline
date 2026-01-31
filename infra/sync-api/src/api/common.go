package api

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/events"
)

type ErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func CreateErrorResponse(statusCode int, code, message string) (events.APIGatewayProxyResponse, error) {
	errResp := ErrorResponse{}
	errResp.Error.Code = code
	errResp.Error.Message = message

	body, _ := json.Marshal(errResp)
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}, nil
}
