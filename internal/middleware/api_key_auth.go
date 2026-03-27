package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/joe/distributed-rate-limiter/internal/auth"
	"github.com/joe/distributed-rate-limiter/internal/handlers"
)

type ProtectedAPIKeyResolver interface {
	ResolveActiveByRawKey(ctx context.Context, rawKey string) (auth.APIKey, error)
}

func APIKeyAuth(resolver ProtectedAPIKeyResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rawKey := strings.TrimSpace(r.Header.Get("X-API-Key"))
			if rawKey == "" {
				handlers.WriteUnauthorized(w, "missing_api_key", "X-API-Key header is required")
				return
			}

			apiKey, err := resolver.ResolveActiveByRawKey(r.Context(), rawKey)
			if err != nil {
				switch {
				case errors.Is(err, auth.ErrInvalidRawAPIKey), errors.Is(err, auth.ErrAPIKeyNotFound):
					handlers.WriteUnauthorized(w, "invalid_api_key", "api key is invalid or inactive")
				default:
					handlers.WriteServiceUnavailable(w, "api_key_resolution_unavailable", "api key resolution is currently unavailable")
				}
				return
			}

			next.ServeHTTP(w, r.WithContext(auth.WithAPIKey(r.Context(), apiKey)))
		})
	}
}
