package routes

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/joe/distributed-rate-limiter/internal/audit"
	"github.com/joe/distributed-rate-limiter/internal/auth"
	"github.com/joe/distributed-rate-limiter/internal/config"
	"github.com/joe/distributed-rate-limiter/internal/handlers"
	appmiddleware "github.com/joe/distributed-rate-limiter/internal/middleware"
	"github.com/joe/distributed-rate-limiter/internal/policies"
	"github.com/joe/distributed-rate-limiter/internal/redisstate"
)

type Dependencies struct {
	APIKeys   *handlers.APIKeysHandler
	Policies  *handlers.PoliciesHandler
	Inspector *handlers.InspectorHandler
	Metrics   *handlers.MetricsHandler
	Protected *handlers.ProtectedHandler

	BlockedAuditor   *audit.Service
	ProtectedAPIKeys *auth.APIKeyService
	PolicyResolver   *policies.Resolver
	BucketStore      *redisstate.BucketStore
}

func New(cfg config.Config, logger *slog.Logger, version string, startedAt time.Time, dependencies Dependencies) http.Handler {
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
		if dependencies.APIKeys != nil {
			r.Get("/api-keys", dependencies.APIKeys.List)
			r.Post("/api-keys", dependencies.APIKeys.Create)
			r.Post("/api-keys/{apiKeyID}/deactivate", dependencies.APIKeys.Deactivate)
		} else {
			r.Get("/api-keys", stubHandler.NotImplemented("list API keys"))
			r.Post("/api-keys", stubHandler.NotImplemented("create API key"))
			r.Post("/api-keys/{apiKeyID}/deactivate", stubHandler.NotImplemented("deactivate API key"))
		}
		if dependencies.Policies != nil {
			r.Get("/policies", dependencies.Policies.List)
			r.Post("/policies", dependencies.Policies.Create)
			r.Put("/policies/{policyID}", dependencies.Policies.Update)
			r.Post("/policies/{policyID}/deactivate", dependencies.Policies.Deactivate)
		} else {
			r.Get("/policies", stubHandler.NotImplemented("list policies"))
			r.Post("/policies", stubHandler.NotImplemented("create policy"))
			r.Put("/policies/{policyID}", stubHandler.NotImplemented("update policy"))
			r.Post("/policies/{policyID}/deactivate", stubHandler.NotImplemented("deactivate policy"))
		}
		if dependencies.Inspector != nil {
			r.Get("/inspect/effective-policy", dependencies.Inspector.EffectivePolicy)
			r.Get("/inspect/bucket", dependencies.Inspector.Bucket)
		} else {
			r.Get("/inspect/effective-policy", stubHandler.NotImplemented("inspect effective policy"))
			r.Get("/inspect/bucket", stubHandler.NotImplemented("inspect bucket state"))
		}
		if dependencies.Metrics != nil {
			r.Get("/metrics/summary", dependencies.Metrics.Summary)
		} else {
			r.Get("/metrics/summary", stubHandler.NotImplemented("inspect summary metrics"))
		}
	})

	for _, definition := range ProtectedRoutes() {
		if dependencies.Protected != nil && dependencies.ProtectedAPIKeys != nil && dependencies.PolicyResolver != nil && dependencies.BucketStore != nil {
			protected := appmiddleware.APIKeyAuth(dependencies.ProtectedAPIKeys)(
				appmiddleware.EnforceRateLimit(definition.ID, definition.Cost, dependencies.PolicyResolver, dependencies.BucketStore, dependencies.BlockedAuditor, time.Now)(
					dependencies.Protected.Route(definition.ID, definition.Cost),
				),
			)
			router.Method(definition.Method, definition.Path, protected)
			continue
		}

		router.MethodFunc(definition.Method, definition.Path, stubHandler.ProtectedRoute(definition.ID, definition.Cost))
	}

	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		handlers.WriteNotFound(w)
	})

	return router
}
