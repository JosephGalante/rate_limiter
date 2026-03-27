package middleware

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/joe/distributed-rate-limiter/internal/handlers"
)

func AdminAuth(expectedToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := bearerToken(r.Header.Get("Authorization"))
			if token == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="admin"`)
				handlers.WriteUnauthorized(w, "missing_admin_token", "admin bearer token is required")
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(expectedToken)) != 1 {
				w.Header().Set("WWW-Authenticate", `Bearer realm="admin"`)
				handlers.WriteUnauthorized(w, "invalid_admin_token", "admin bearer token is invalid")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(header string) string {
	const prefix = "Bearer "

	if !strings.HasPrefix(header, prefix) {
		return ""
	}

	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
}
