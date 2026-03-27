package middleware

import (
	"log/slog"
	"net/http"

	"github.com/joe/distributed-rate-limiter/internal/handlers"
)

func Recoverer(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if recovered := recover(); recovered != nil {
					logger.Error("panic recovered", slog.Any("panic", recovered))
					handlers.WriteInternalServerError(w)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
