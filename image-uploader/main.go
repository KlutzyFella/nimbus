package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	// AWS
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// defaultMaxUploadBytes caps a single upload request body. The whole
// payload is buffered in memory, so an unbounded body is a trivial
// denial of service.
const defaultMaxUploadBytes = 15 << 20 // 15 MiB

// Worker-call defaults. The worker address is a K8s Service DNS name in
// the cluster and overrideable for local development and tests.
const (
	defaultWorkerURL     = "http://worker-service:8081/process"
	defaultWorkerTimeout = 5 * time.Second
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
	// MaxUploadBytes bounds the request body; tests may shrink it.
	MaxUploadBytes int64
	// WorkerURL is where the processor is notified; WORKER_URL overrides it.
	WorkerURL string
	// Worker bounds the notification call so a hung worker cannot
	// leak the request goroutine forever.
	Worker *http.Client
}

// NewServerFromEnv builds the production Server. It returns an error
// instead of panicking so main can fail with a clear message.
func NewServerFromEnv() (*Server, error) {
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %w", err)
	}

	workerURL := os.Getenv("WORKER_URL")
	if workerURL == "" {
		workerURL = defaultWorkerURL
	}

	return &Server{
		S3:             s3.NewFromConfig(cfg),
		Bucket:         os.Getenv("S3_BUCKET_NAME"),
		MaxUploadBytes: defaultMaxUploadBytes,
		WorkerURL:      workerURL,
		Worker:         &http.Client{Timeout: defaultWorkerTimeout},
	}, nil
}

// SanitizeFilename returns a safe filename stripped of path and dangerous characters.
func SanitizeFilename(filename string) string {
	base := filepath.Base(filename)
	re := regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	sanitized := re.ReplaceAllString(base, "_")
	return sanitized
}

// notifyWorker tells the processor about a new object and reports the
// outcome, so a failure can be surfaced instead of vanishing in a log.
func (s *Server) notifyWorker(ctx context.Context, bucket, key string) error {
	workerPayload, err := json.Marshal(map[string]string{"bucket": bucket, "key": key})
	if err != nil {
		return fmt.Errorf("encode worker payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.WorkerURL, bytes.NewReader(workerPayload))
	if err != nil {
		return fmt.Errorf("build worker request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Worker.Do(req)
	if err != nil {
		return fmt.Errorf("call worker service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("worker service returned %s", resp.Status)
	}
	return nil
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

	// Bound the request body: the whole payload is buffered in memory.
	r.Body = http.MaxBytesReader(w, r.Body, s.MaxUploadBytes)

	// Parse the incoming JSON
	var upload UploadRequest
	if err := json.NewDecoder(r.Body).Decode(&upload); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, `{"error":"Payload too large"}`, http.StatusRequestEntityTooLarge)
			return
		}
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
	if sanitizedFileName == "" || sanitizedFileName == "." {
		http.Error(w, `{"error":"Invalid filename"}`, http.StatusBadRequest)
		return
	}
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

	// Notify the worker synchronously, bounded by the client timeout, so
	// the outcome is observable. The S3 upload already succeeded, so the
	// status stays 200 with a real URL — but a notification failure is
	// disclosed as a warning instead of vanishing into a log line.
	response := map[string]string{}
	if err := s.notifyWorker(r.Context(), s.Bucket, sanitizedFileName); err != nil {
		log.Printf("Worker notification failed for s3://%s/%s: %v", s.Bucket, sanitizedFileName, err)
		response["warning"] = "uploaded, but the processor was not notified"
	}

	// Return the URL of the uploaded file
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.Bucket, sanitizedFileName)
	response["url"] = url
	responseBody, _ := json.Marshal(response)

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
