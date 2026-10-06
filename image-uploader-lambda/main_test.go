package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type fakeS3 struct {
	putErr   error
	lastKey  string
	lastBody []byte
}

func (f *fakeS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if f.putErr != nil {
		return nil, f.putErr
	}
	if in.Key != nil {
		f.lastKey = *in.Key
	}
	return &s3.PutObjectOutput{}, nil
}

type fakeSQS struct {
	sendErr  error
	lastBody string
}

func (f *fakeSQS) SendMessage(_ context.Context, in *sqs.SendMessageInput, _ ...func(*sqs.Options)) (*sqs.SendMessageOutput, error) {
	if f.sendErr != nil {
		return nil, f.sendErr
	}
	if in.MessageBody != nil {
		f.lastBody = *in.MessageBody
	}
	return &sqs.SendMessageOutput{}, nil
}

func uploadRequest() events.APIGatewayProxyRequest {
	return events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Body:       `{"filename":"a.txt","filedata":"aGk=","contenttype":"text/plain"}`,
	}
}

func TestSQSErrorSurfacedAsWarning(t *testing.T) {
	// SQS is down but S3 works: the file is stored and the URL is real,
	// so the status stays 200 — but the failure must be disclosed.
	srv := &Server{
		S3:       &fakeS3{},
		SQS:      &fakeSQS{sendErr: errors.New("queue unavailable")},
		Bucket:   "test-bucket",
		QueueURL: "https://sqs.test/queue",
	}

	resp, err := srv.handler(context.Background(), uploadRequest())
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if got["url"] == "" {
		t.Errorf("response lost the url on SQS failure: %v", got)
	}
	if got["warning"] == "" {
		t.Errorf("response hides the SQS failure (no warning): %v", got)
	}
}

func TestSQSSuccessHasNoWarning(t *testing.T) {
	sqsFake := &fakeSQS{}
	srv := &Server{
		S3:       &fakeS3{},
		SQS:      sqsFake,
		Bucket:   "test-bucket",
		QueueURL: "https://sqs.test/queue",
	}

	resp, err := srv.handler(context.Background(), uploadRequest())
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, resp.Body)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(resp.Body), &got); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	if _, ok := got["warning"]; ok {
		t.Errorf("healthy SQS produced a warning: %v", got)
	}
	if sqsFake.lastBody == "" {
		t.Errorf("SQS message was never sent")
	}
}
