package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type ProcessRequest struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func processHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Log the message
	log.Printf("Worker received processing request for s3://%s/%s", req.Bucket, req.Key)

	// Send a simple "OK" response back to the uploader
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Request received"))
}

func main() {
	http.HandleFunc("/process", processHandler)
	log.Println("Starting server on port 8081")
	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
