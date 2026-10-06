package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeS3 records PutObject calls without touching AWS.
type fakeS3 struct {
	putErr   error
	lastKey  string
	lastBody []byte
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	body, _ := io.ReadAll(in.Body)
	if in.Key != nil {
		f.lastKey = *in.Key
	}
	f.lastBody = body
	return &s3.PutObjectOutput{}, nil
}

func testServer(fake *fakeS3) *Server {
	return &Server{
		S3:             fake,
		Bucket:         "test-bucket",
		MaxUploadBytes: defaultMaxUploadBytes,
		WorkerURL:      "http://127.0.0.1:1/process", // unroutable; worker tests override
		Worker:         &http.Client{Timeout: defaultWorkerTimeout},
	}
}

func postUpload(t *testing.T, srv *Server, payload map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.uploadHandler(rec, req)
	return rec
}

func validPayload(filename string) map[string]string {
	return map[string]string{
		"filename":    filename,
		"filedata":    base64.StdEncoding.EncodeToString([]byte("hello")),
		"contenttype": "text/plain",
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"photo.jpg":        "photo.jpg",
		"../../etc/passwd": "passwd",
		"my photo (1).JPG": "my_photo__1_.JPG",
		"/":                "_",
		// filepath.Base("") == "."; the handler must reject this (see TestEmptyFilename).
		"":         ".",
		"x.tar.gz": "x.tar.gz",
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEmptyFilename(t *testing.T) {
	// SanitizeFilename("") and (".") both yield ".", which would
	// otherwise become a nonsense S3 key with a URL ending in "/.".
	for _, name := range []string{"", "."} {
		fake := &fakeS3{}
		rec := postUpload(t, testServer(fake), validPayload(name))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("filename %q: status = %d, want 400 (body: %s)", name, rec.Code, rec.Body.String())
		}
		if fake.lastKey != "" {
			t.Errorf("filename %q: reached S3 with key %q, want no upload", name, fake.lastKey)
		}
	}
}

func TestOversizedPayloadRejected(t *testing.T) {
	// One MiB over the 15 MiB default request cap. Must be rejected
	// before anything is buffered into S3.
	fake := &fakeS3{}
	payload := validPayload("big.bin")
	payload["filedata"] = strings.Repeat("A", 16<<20)
	rec := postUpload(t, testServer(fake), payload)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413 (body: %.120s)", rec.Code, rec.Body.String())
	}
	if fake.lastKey != "" {
		t.Errorf("oversized payload reached S3 with key %q", fake.lastKey)
	}
}

func TestWorkerNotificationHonorsWorkerURL(t *testing.T) {
	var hits int
	var gotBody []byte
	ws := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		gotBody, _ = io.ReadAll(r.Body)
	}))
	t.Cleanup(ws.Close)

	// Point the handler at the test worker through the environment.
	// Pre-fix the handler ignores this and dials a hardcoded K8s DNS
	// name, so the test worker is never hit.
	t.Setenv("WORKER_URL", ws.URL+"/process")
	t.Setenv("S3_BUCKET_NAME", "test-bucket")
	srv, err := NewServerFromEnv()
	if err != nil {
		t.Fatalf("NewServerFromEnv: %v", err)
	}
	srv.S3 = &fakeS3{}

	rec := postUpload(t, srv, validPayload("w.txt"))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if hits != 1 {
		t.Fatalf("worker hits = %d, want 1 (WORKER_URL was ignored)", hits)
	}
	var payload map[string]string
	if err := json.Unmarshal(gotBody, &payload); err != nil {
		t.Fatalf("worker body is not JSON: %v", err)
	}
	if payload["key"] != "w.txt" || payload["bucket"] == "" {
		t.Errorf("worker payload = %v, want bucket+key", payload)
	}
}

func TestUploadRoundTrip(t *testing.T) {
	fake := &fakeS3{}
	rec := postUpload(t, testServer(fake), validPayload("hello.txt"))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if got["url"] == "" {
		t.Errorf("response has no url: %v", got)
	}
	if fake.lastKey != "hello.txt" {
		t.Errorf("S3 key = %q, want %q", fake.lastKey, "hello.txt")
	}
	if string(fake.lastBody) != "hello" {
		t.Errorf("S3 body = %q, want %q", fake.lastBody, "hello")
	}
}
