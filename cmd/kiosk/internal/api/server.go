package api

import (
	"fmt"
	"net/http"
)

const apiPrefix = "/wpe-webkit-kiosk/api/v1"

// NewServer creates an HTTP server with versioned API routing and auth middleware.
func NewServer(port, token string) *http.Server {
	mux := http.NewServeMux()
	registerRoutes(mux, token)

	return &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%s", port),
		Handler: mux,
	}
}
