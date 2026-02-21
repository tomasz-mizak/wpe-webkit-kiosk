package api

import (
	"crypto/subtle"
	"net/http"
)

// authMiddleware validates the X-Api-Key header against the configured token.
func authMiddleware(token string, next http.Handler) http.Handler {
	tokenBytes := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("X-Api-Key")
		if key == "" {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Missing X-Api-Key header")
			return
		}
		if subtle.ConstantTimeCompare([]byte(key), tokenBytes) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized", "Invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}
