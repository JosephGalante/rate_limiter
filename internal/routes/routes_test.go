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
)

func TestHealthRoute(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC())

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
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC())

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", recorder.Code)
	}
}

func TestAdminRoutesAcceptConfiguredBearerToken(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC())

	request := httptest.NewRequest(http.MethodGet, "/api/admin/ping", nil)
	request.Header.Set("Authorization", "Bearer test-admin-token")
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", recorder.Code)
	}
}

func TestProtectedRouteScaffoldIsMounted(t *testing.T) {
	router := New(testConfig(), testLogger(), "test", time.Unix(0, 0).UTC())

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
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
