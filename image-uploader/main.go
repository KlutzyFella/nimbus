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

var s3Client *s3.Client

func init() {
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic("unable to load SDK config, " + err.Error())
	}

	// Create an S3 client
	s3Client = s3.NewFromConfig(cfg)
}

// SanitizeFilename returns a safe filename stripped of path and dangerous characters.
func SanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := re.ReplaceAllString(base, "_")
	return sanitized
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
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
	bucketName := os.Getenv("S3_BUCKET_NAME")
	_, err = s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
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
	workerPayload := fmt.Sprintf(`{"bucket":"%s", "key":"%s"}`, bucketName, sanitizedFileName)
	go func() {
		// The URL "http://localhost:8081/process" is for local testing. In Kubernetes, this will be a service name.
		_, err := http.Post("http://worker-service:8081/process", "application/json", bytes.NewBufferString(workerPayload))
		if err != nil {
			log.Printf("Failed to call worker service: %v", err)
		}
	}()

	// Return the URL of the uploaded file
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", bucketName, sanitizedFileName)
	responseBody, _ := json.Marshal(map[string]string{"url": url})

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func main() {
	// Start the uploader service
	http.HandleFunc("/upload", uploadHandler)

	log.Println("Uploader service starting on port 8080...")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
