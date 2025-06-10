package main

import (
	// Standard library
	"context"
	"encoding/json"

	// AWS Lambda
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

// Struct to parse incoming JSON
type UploadRequest struct {
	FileName string `json:"filename"`
	FileData string `json:"filedata"` // base64 string
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	//
	var upload UploadRequest
	err := json.Unmarshal([]byte(request.Body), &upload)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: `{"error":"Invalid request"}`}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       `{"message": "Hello from Lambda"}`,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
	}, nil
}

// Main function to start the Lambda function
func main() {
	lambda.Start(handler)
}
