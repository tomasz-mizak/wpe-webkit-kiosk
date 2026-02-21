package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setupTestServer(token string) *http.ServeMux {
	mux := http.NewServeMux()
	registerRoutes(mux, token)
	return mux
}

func doRequest(mux http.Handler, method, path, token string, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("X-Api-Key", token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRoutes_UnauthReturns401(t *testing.T) {
	mux := setupTestServer("secret")
	rec := doRequest(mux, "GET", "/wpe-webkit-kiosk/api/v1/status", "", "")
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRoutes_InvalidPath404(t *testing.T) {
	mux := setupTestServer("secret")
	rec := doRequest(mux, "GET", "/nonexistent", "secret", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestNavigate_MissingURL(t *testing.T) {
	mux := setupTestServer("secret")
	rec := doRequest(mux, "POST", "/wpe-webkit-kiosk/api/v1/navigate", "secret", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestNavigate_InvalidJSON(t *testing.T) {
	mux := setupTestServer("secret")
	rec := doRequest(mux, "POST", "/wpe-webkit-kiosk/api/v1/navigate", "secret", `not json`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestClear_InvalidScope(t *testing.T) {
	mux := setupTestServer("secret")
	rec := doRequest(mux, "POST", "/wpe-webkit-kiosk/api/v1/clear", "secret", `{"scope": "invalid"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var env envelope
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error == nil || env.Error.Code != "invalid_scope" {
		t.Errorf("expected error code 'invalid_scope', got %+v", env.Error)
	}
}

func TestConfigSet_ForbiddenToken(t *testing.T) {
	mux := setupTestServer("secret")
	rec := doRequest(mux, "PUT", "/wpe-webkit-kiosk/api/v1/config", "secret", `{"key": "API_TOKEN", "value": "hack"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var env envelope
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error == nil || env.Error.Code != "forbidden_key" {
		t.Errorf("expected error code 'forbidden_key', got %+v", env.Error)
	}
}

func TestConfigSet_UnknownKey(t *testing.T) {
	mux := setupTestServer("secret")
	rec := doRequest(mux, "PUT", "/wpe-webkit-kiosk/api/v1/config", "secret", `{"key": "NONEXISTENT", "value": "x"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}

	var env envelope
	json.Unmarshal(rec.Body.Bytes(), &env)
	if env.Error == nil || env.Error.Code != "unknown_key" {
		t.Errorf("expected error code 'unknown_key', got %+v", env.Error)
	}
}
