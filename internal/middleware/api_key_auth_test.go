package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/joe/distributed-rate-limiter/internal/auth"
)

func TestAPIKeyAuthRejectsMissingHeader(t *testing.T) {
	middleware := APIKeyAuth(fakeAPIKeyResolver{})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/ping", nil)

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected next handler not to run")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestAPIKeyAuthRejectsInvalidKey(t *testing.T) {
	middleware := APIKeyAuth(fakeAPIKeyResolver{err: auth.ErrAPIKeyNotFound})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/ping", nil)
	request.Header.Set("X-API-Key", "invalid")

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected next handler not to run")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recorder.Code)
	}
}

func TestAPIKeyAuthStoresAPIKeyInContext(t *testing.T) {
	expected := auth.APIKey{ID: uuid.MustParse("4ee491f1-2d45-4bf9-ba45-f6b9a929d933")}
	middleware := APIKeyAuth(fakeAPIKeyResolver{apiKey: expected})
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/ping", nil)
	request.Header.Set("X-API-Key", "rls_live_test")

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey, ok := auth.APIKeyFromContext(r.Context())
		if !ok {
			t.Fatalf("expected api key in context")
		}
		if apiKey.ID != expected.ID {
			t.Fatalf("expected api key id %s, got %s", expected.ID, apiKey.ID)
		}

		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

type fakeAPIKeyResolver struct {
	apiKey auth.APIKey
	err    error
}

func (f fakeAPIKeyResolver) ResolveActiveByRawKey(context.Context, string) (auth.APIKey, error) {
	if f.err != nil {
		return auth.APIKey{}, f.err
	}

	if f.apiKey.ID == uuid.Nil {
		return auth.APIKey{}, errors.New("missing api key fixture")
	}

	return f.apiKey, nil
}
