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

// S3Putter and SQSSender are the narrow client subsets the handler needs.
// The real SDK clients satisfy them; tests substitute fakes.
type S3Putter interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

type SQSSender interface {
	SendMessage(ctx context.Context, params *sqs.SendMessageInput, optFns ...func(*sqs.Options)) (*sqs.SendMessageOutput, error)
}

// Server holds the handler's dependencies so it is testable without AWS.
type Server struct {
	S3       S3Putter
	SQS      SQSSender
	Bucket   string
	QueueURL string
}

// NewServerFromEnv builds the production Server. It returns an error
// instead of panicking so main can fail with a clear message.
func NewServerFromEnv() (*Server, error) {
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	return &Server{
		S3:       s3.NewFromConfig(cfg),
		SQS:      sqs.NewFromConfig(cfg),
		Bucket:   os.Getenv("S3_BUCKET_NAME"),
		QueueURL: os.Getenv("SQS_QUEUE_URL"),
	}, nil
}

// SanitizeFilename returns a safe filename stripped of path and dangerous characters.
func SanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := re.ReplaceAllString(base, "_")
	return sanitized
}

func (s *Server) handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
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
	_, err = s.S3.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(sanitizedFileName),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		fmt.Println("Upload failed: ", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Headers: headers, Body: `{"error":"Upload failed"}`}, nil
	}

	// Send the file metadata to SQS. The S3 upload already succeeded, so
	// the status stays 200 with a real URL — but a queue failure is
	// disclosed as a warning instead of vanishing into a log line.
	messageBody := fmt.Sprintf(`{"bucket":"%s", "key":"%s"}`, s.Bucket, sanitizedFileName)
	response := map[string]string{}
	_, err = s.SQS.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(s.QueueURL),
		MessageBody: aws.String(messageBody),
	})
	if err != nil {
		fmt.Printf("Failed to send message to SQS: %v\n", err)
		response["warning"] = "uploaded, but the processor was not notified"
	}

	// Return the URL of the uploaded file
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.Bucket, sanitizedFileName)
	response["url"] = url
	responseBody, _ := json.Marshal(response)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(responseBody),
		Headers:    headers,
	}, nil
}

func main() {
	srv, err := NewServerFromEnv()
	if err != nil {
		panic(fmt.Sprintf("failed to configure lambda handler: %v", err))
	}
	lambda.Start(srv.handler)
}
