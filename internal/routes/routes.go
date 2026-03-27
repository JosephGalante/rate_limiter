package routes

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/joe/distributed-rate-limiter/internal/config"
	"github.com/joe/distributed-rate-limiter/internal/handlers"
	appmiddleware "github.com/joe/distributed-rate-limiter/internal/middleware"
)

func New(cfg config.Config, logger *slog.Logger, version string, startedAt time.Time) http.Handler {
	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(chimiddleware.RealIP)
	router.Use(chimiddleware.Timeout(30 * time.Second))
	router.Use(appmiddleware.Recoverer(logger))

	healthHandler := handlers.NewHealthHandler(startedAt, version)
	stubHandler := handlers.NewStubHandler()

	router.Get("/healthz", healthHandler.Live)

	router.Route("/api/admin", func(r chi.Router) {
		r.Use(appmiddleware.AdminAuth(cfg.Admin.Token))
		r.Get("/ping", stubHandler.AdminPing)
		r.Get("/api-keys", stubHandler.NotImplemented("list API keys"))
		r.Post("/api-keys", stubHandler.NotImplemented("create API key"))
		r.Post("/api-keys/{apiKeyID}/deactivate", stubHandler.NotImplemented("deactivate API key"))
		r.Get("/policies", stubHandler.NotImplemented("list policies"))
		r.Post("/policies", stubHandler.NotImplemented("create policy"))
		r.Put("/policies/{policyID}", stubHandler.NotImplemented("update policy"))
		r.Post("/policies/{policyID}/deactivate", stubHandler.NotImplemented("deactivate policy"))
		r.Get("/inspect/effective-policy", stubHandler.NotImplemented("inspect effective policy"))
		r.Get("/inspect/bucket", stubHandler.NotImplemented("inspect bucket state"))
		r.Get("/metrics/summary", stubHandler.NotImplemented("inspect summary metrics"))
	})

	for _, definition := range ProtectedRoutes() {
		router.MethodFunc(definition.Method, definition.Path, stubHandler.ProtectedRoute(definition.ID, definition.Cost))
	}

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		handlers.WriteNotFound(w)
	})

	return router
}
