package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON_SuccessEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, 200, map[string]string{"key": "value"})

	if rec.Code != 200 {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if env.Error != nil {
		t.Errorf("expected error to be nil, got %+v", env.Error)
	}

	if env.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestWriteError_ErrorEnvelope(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, 401, "unauthorized", "Missing key")

	if rec.Code != 401 {
		t.Errorf("expected 401, got %d", rec.Code)
	}

	var env envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if env.Data != nil {
		t.Errorf("expected data to be nil, got %+v", env.Data)
	}

	if env.Error == nil {
		t.Fatal("expected error to be non-nil")
	}

	if env.Error.Code != "unauthorized" {
		t.Errorf("expected code 'unauthorized', got %s", env.Error.Code)
	}

	if env.Error.Message != "Missing key" {
		t.Errorf("expected message 'Missing key', got %s", env.Error.Message)
	}
}
