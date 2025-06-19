package middleware

import (
	"net/http"
	"strings"
)

// RequireContentType returns middleware that validates Content-Type header
func RequireContentType(contentTypes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(contentTypes) == 0 {
				next.ServeHTTP(w, r)
				return
			}

			requestContentType := r.Header.Get("Content-Type")
			// Handle content types with charset (e.g., "application/json; charset=utf-8")
			if idx := strings.Index(requestContentType, ";"); idx != -1 {
				requestContentType = strings.TrimSpace(requestContentType[:idx])
			}

			for _, ct := range contentTypes {
				if requestContentType == ct {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, "Content-Type must be "+strings.Join(contentTypes, " or "), http.StatusBadRequest)
		})
	}
}
