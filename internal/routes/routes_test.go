package routes

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joe/distributed-rate-limiter/internal/config"
	"github.com/joe/distributed-rate-limiter/internal/handlers"
)

func TestHealthRoute(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC(), Dependencies{})

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), `"status":"ok"`) {
		t.Fatalf("expected health response body, got %s", recorder.Body.String())
	}
}

func TestAdminRoutesRequireBearerToken(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC(), Dependencies{})

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestAdminRoutesAcceptConfiguredBearerToken(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC(), Dependencies{})

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	request.Header.Set("Authorization", "Bearer test-admin-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestPublicConfigRouteIsMounted(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC(), Dependencies{
		PublicConfig: handlers.NewPublicConfigHandler(config.DemoConfig{}, nil),
	})

	request := httptest.NewRequest(http.MethodGet, "/api/public/config", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestDemoModeDisablesPolicyMutationRoutes(t *testing.T) {
	cfg := testConfig()
	cfg.Demo.PublicMode = true

	router := New(cfg, testLogger(), "test", time.Unix(0, 0).UTC(), Dependencies{
		Policies: handlers.NewPoliciesHandler(nil),
	})

	request := httptest.NewRequest(http.MethodPost, "/api/admin/policies", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer test-admin-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected status 403, got %d", recorder.Code)
	}
}

func TestProtectedRouteScaffoldIsMounted(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC(), Dependencies{})

	request := httptest.NewRequest(http.MethodPost, "/api/protected/orders", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}

	if !strings.Contains(recorder.Body.String(), `"route_id":"orders"`) {
		t.Fatalf("expected protected route metadata, got %s", recorder.Body.String())
	}
}

func testConfig() config.Config {
	return config.Config{
		AppEnv: "test",
		Server: config.ServerConfig{
			Addr: ":8080",
		},
		Admin: config.AdminConfig{
			Token: "test-admin-token",
		},
		Demo: config.DemoConfig{},
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
