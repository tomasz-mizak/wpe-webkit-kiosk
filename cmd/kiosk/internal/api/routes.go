package api

import "net/http"

// registerRoutes sets up all API v1 routes with auth middleware.
func registerRoutes(mux *http.ServeMux, token string) {
	v1 := http.NewServeMux()

	v1.HandleFunc("GET /status", handleStatus)
	v1.HandleFunc("POST /navigate", handleNavigate)
	v1.HandleFunc("POST /reload", handleReload)
	v1.HandleFunc("GET /config", handleConfigGet)
	v1.HandleFunc("PUT /config", handleConfigSet)
	v1.HandleFunc("POST /clear", handleClear)
	v1.HandleFunc("GET /extensions", handleExtensionsList)
	v1.HandleFunc("POST /extensions/{name}/enable", handleExtensionEnable)
	v1.HandleFunc("POST /extensions/{name}/disable", handleExtensionDisable)
	v1.HandleFunc("POST /restart", handleRestart)
	v1.HandleFunc("GET /system", handleSystem)

	mux.Handle(apiPrefix+"/", authMiddleware(token,
		http.StripPrefix(apiPrefix, v1)))

	// Docs endpoints — no auth required
	mux.HandleFunc("GET "+apiPrefix+"/docs", handleDocs)
	mux.HandleFunc("GET "+apiPrefix+"/docs/openapi.yaml", handleOpenAPISpec)
}
