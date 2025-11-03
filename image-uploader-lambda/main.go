package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	// AWS Lambda
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// Struct to parse incoming JSON
type UploadRequest struct {
	FileName    string `json:"filename"`
	FileData    string `json:"filedata"` // base64 string
	ContentType string `json:"contenttype"`
}

var s3Client *s3.Client
var sqsClient *sqs.Client

func init() {
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}

	// Create an S3 client
	s3Client = s3.NewFromConfig(cfg)
	// Create an SQS client
	sqsClient = sqs.NewFromConfig(cfg)
}

// SanitizeFilename returns a safe filename stripped of path and dangerous characters.
func SanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := re.ReplaceAllString(base, "_")
	return sanitized
}

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "POST, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type, X-Amz-Date, Authorization, X-Api-Key, X-Amz-Security-Token",
	}

	if request.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    headers,
			Body:       "",
		}, nil
	}

	// Parse the incoming JSON
	var upload UploadRequest
	err := json.Unmarshal([]byte(request.Body), &upload)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Headers: headers, Body: `{"error":"Invalid request"}`}, nil
	}

	// Decode the base64 file data
	fileBytes, err := base64.StdEncoding.DecodeString(upload.FileData)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Headers: headers, Body: `{"error":"Invalid base64"}`}, nil
	}

	// Sanitize the filename and get the content type
	sanitizedFileName := SanitizeFilename(upload.FileName)
	contentType := upload.ContentType
	if contentType == "" {
		contentType = http.DetectContentType(fileBytes)
	}

	// Upload the file to S3
	bucketName := os.Getenv("S3_BUCKET_NAME")
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(sanitizedFileName),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		fmt.Println("Upload failed: ", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Headers: headers, Body: `{"error":"Upload failed"}`}, nil
	}

	// Send the file metadata to SQS
	sqsQueueURL := os.Getenv("SQS_QUEUE_URL")
	messageBody := fmt.Sprintf(`{"bucket":"%s", "key":"%s"}`, bucketName, sanitizedFileName)

	_, err = sqsClient.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(sqsQueueURL),
		MessageBody: aws.String(messageBody),
	})
	if err != nil {
		fmt.Printf("Failed to send message to SQS: %v\n", err)
	}

	// Return the URL of the uploaded file
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, sanitizedFileName)
	responseBody, _ := json.Marshal(map[string]string{"url": url})

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(responseBody),
		Headers:    headers,
	}, nil
}

func main() {
	lambda.Start(handler)
}
