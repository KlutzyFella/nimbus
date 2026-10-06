package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	// AWS
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Struct to parse incoming JSON
type UploadRequest struct {
	FileName    string `json:"filename"`
	FileData    string `json:"filedata"` // base64 string
	ContentType string `json:"contenttype"`
}

// S3Putter is the subset of the S3 client the uploader needs.
// *s3.Client satisfies it; tests substitute a fake.
type S3Putter interface {
	PutObject(ctx context.Context, params *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
}

// Server holds the uploader's dependencies so handlers are testable
// without AWS credentials or a live worker service.
type Server struct {
	S3     S3Putter
	Bucket string
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
		S3:     s3.NewFromConfig(cfg),
		Bucket: os.Getenv("S3_BUCKET_NAME"),
	}, nil
}

// SanitizeFilename returns a safe filename stripped of path and dangerous characters.
func SanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := re.ReplaceAllString(base, "_")
	return sanitized
}

func (s *Server) uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Set CORS headers for all responses
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse the incoming JSON
	var upload UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&upload); err != nil {
		http.Error(w, `{"error":"Invalid request"}`, http.StatusBadRequest)
		return
	}

	// Decode the base64 file data
	fileBytes, err := base64.StdEncoding.DecodeString(upload.FileData)
	if err != nil {
		http.Error(w, `{"error":"Invalid base64"}`, http.StatusBadRequest)
		return
	}

	// Sanitize the filename and get the content type
	sanitizedFileName := SanitizeFilename(upload.FileName)
	contentType := upload.ContentType
	if contentType == "" {
		contentType = http.DetectContentType(fileBytes)
	}

	// Upload the file to S3
	_, err = s.S3.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(s.Bucket),
		Key:         aws.String(sanitizedFileName),
		Body:        bytes.NewReader(fileBytes),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		fmt.Println("Upload failed: ", err)
		http.Error(w, `{"error":"Upload failed"}`, http.StatusInternalServerError)
		return
	}

	// Instead of sending a message to SQS, we now make a direct HTTP call to our worker service.
	workerPayload := fmt.Sprintf(`{"bucket":"%s", "key":"%s"}`, s.Bucket, sanitizedFileName)
	go func() {
		// The URL "http://localhost:8081/process" is for local testing. In Kubernetes, this will be a service name.
		_, err := http.Post("http://worker-service:8081/process", "application/json", bytes.NewBufferString(workerPayload))
		if err != nil {
			log.Printf("Failed to call worker service: %v", err)
		}
	}()

	// Return the URL of the uploaded file
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.Bucket, sanitizedFileName)
	responseBody, _ := json.Marshal(map[string]string{"url": url})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func main() {
	// Start the uploader service
	srv, err := NewServerFromEnv()
	if err != nil {
		log.Fatalf("Failed to configure uploader service: %v", err)
	}
	http.HandleFunc("/upload", srv.uploadHandler)

	log.Println("Uploader service starting on port 8080...")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
