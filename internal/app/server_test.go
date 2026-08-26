package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestUploadAndMCPQuery(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	server := NewServer(store, "secret", slog.New(slog.NewTextHandler(io.Discard, nil)))
	value := 123.0
	batch := UploadBatch{DeviceID: "iphone", Type: "HKQuantityTypeIdentifierStepCount", Samples: []Sample{{ID: "one", Type: "HKQuantityTypeIdentifierStepCount", Kind: "quantity", StartDate: "2026-08-24T10:00:00Z", EndDate: "2026-08-24T11:00:00Z", Value: &value, Unit: "count"}}}
	body, _ := json.Marshal(batch)
	request := httptest.NewRequest(http.MethodPost, "/upload/v1/batches", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("upload: %d %s", response.Code, response.Body.String())
	}
	rpc := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"health_query","arguments":{"type":"HKQuantityTypeIdentifierStepCount","from":"2026-08-24T00:00:00Z"}}}`)
	request = httptest.NewRequest(http.MethodPost, "/mcp", bytes.NewReader(rpc))
	request.Header.Set("Authorization", "Bearer secret")
	request.Header.Set("Accept", "application/json")
	response = httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatalf("mcp: %d %s", response.Code, response.Body.String())
	}
	if !bytes.Contains(response.Body.Bytes(), []byte(`"value":123`)) {
		t.Fatalf("missing sample: %s", response.Body.String())
	}
}

func TestApplyDeletion(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	value := 1.0
	_, err = store.ApplyBatch(context.Background(), UploadBatch{DeviceID: "phone", Samples: []Sample{{ID: "x", Type: "t", Kind: "quantity", StartDate: "2026-01-01T00:00:00Z", EndDate: "2026-01-01T00:00:00Z", Value: &value}}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.ApplyBatch(context.Background(), UploadBatch{DeviceID: "phone", DeletedIDs: []string{"x"}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Deleted != 1 {
		t.Fatalf("deleted=%d", result.Deleted)
	}
}
