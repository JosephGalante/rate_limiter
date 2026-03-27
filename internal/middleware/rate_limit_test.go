package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/joe/distributed-rate-limiter/internal/audit"
	"github.com/joe/distributed-rate-limiter/internal/auth"
	"github.com/joe/distributed-rate-limiter/internal/policies"
	"github.com/joe/distributed-rate-limiter/internal/ratelimit"
	"github.com/joe/distributed-rate-limiter/internal/redisstate"
)

func TestEnforceRateLimitAllowsAndSetsHeaders(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	apiKeyID := uuid.MustParse("f4ad3b1a-5376-4eb8-b18e-d4944bf83e74")

	middleware := EnforceRateLimit("ping", 1, fakeEffectivePolicyResolver{
		resolution: policies.Resolution{
			Policy: policies.Policy{
				ID:                    uuid.MustParse("0ac7368b-4475-44ef-9312-c0f0bb1b4a25"),
				Capacity:              10,
				RefillTokens:          1,
				RefillIntervalSeconds: 60,
			},
			MatchedScopeType:       policies.ScopeGlobal,
			MatchedScopeIdentifier: nil,
			MatchedRoutePattern:    nil,
		},
		found: true,
	}, fakeBucketConsumer{
		decision: ratelimit.Decision{
			Allowed:   true,
			Limit:     10,
			Remaining: 9,
			ResetAt:   now.Add(60 * time.Second),
		},
	}, nil, func() time.Time { return now })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/ping", nil)
	request = request.WithContext(auth.WithAPIKey(request.Context(), auth.APIKey{ID: apiKeyID}))

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if recorder.Header().Get("X-RateLimit-Limit") != "10" {
		t.Fatalf("expected X-RateLimit-Limit to be 10, got %q", recorder.Header().Get("X-RateLimit-Limit"))
	}
	if recorder.Header().Get("X-RateLimit-Remaining") != "9" {
		t.Fatalf("expected X-RateLimit-Remaining to be 9, got %q", recorder.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestEnforceRateLimitBlocksAndSetsRetryAfter(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	apiKeyID := uuid.MustParse("45c660c2-3205-4919-ac3f-126c09b1b730")

	auditor := &fakeBlockedRequestAuditor{}
	middleware := EnforceRateLimit("report", 5, fakeEffectivePolicyResolver{
		resolution: policies.Resolution{
			Policy: policies.Policy{
				Capacity:              5,
				RefillTokens:          1,
				RefillIntervalSeconds: 60,
			},
			MatchedScopeType:    policies.ScopeRoute,
			MatchedRoutePattern: stringPointer("report"),
		},
		found: true,
	}, fakeBucketConsumer{
		decision: ratelimit.Decision{
			Allowed:    false,
			Limit:      5,
			Remaining:  0,
			RetryAfter: 90 * time.Second,
			ResetAt:    now.Add(5 * time.Minute),
		},
	}, auditor, func() time.Time { return now })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/report", nil)
	request = request.WithContext(auth.WithAPIKey(request.Context(), auth.APIKey{ID: apiKeyID}))

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected next handler not to run")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", recorder.Code)
	}
	if recorder.Header().Get("Retry-After") != "90" {
		t.Fatalf("expected Retry-After to be 90, got %q", recorder.Header().Get("Retry-After"))
	}
	if len(auditor.requests) != 1 {
		t.Fatalf("expected one blocked audit record, got %d", len(auditor.requests))
	}
	if auditor.requests[0].RouteID != "report" {
		t.Fatalf("expected blocked audit route report, got %q", auditor.requests[0].RouteID)
	}
}

func TestEnforceRateLimitReturnsServiceUnavailableWithoutPolicy(t *testing.T) {
	apiKeyID := uuid.MustParse("5f64752a-e567-4d20-bc11-cf8def69f4f2")
	middleware := EnforceRateLimit("ping", 1, fakeEffectivePolicyResolver{}, fakeBucketConsumer{}, nil, time.Now)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/ping", nil)
	request = request.WithContext(auth.WithAPIKey(request.Context(), auth.APIKey{ID: apiKeyID}))

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected next handler not to run")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}

type fakeEffectivePolicyResolver struct {
	resolution policies.Resolution
	found      bool
	err        error
}

func (f fakeEffectivePolicyResolver) Resolve(context.Context, policies.ResolveInput) (policies.Resolution, bool, error) {
	return f.resolution, f.found, f.err
}

type fakeBucketConsumer struct {
	decision ratelimit.Decision
	err      error
}

func (f fakeBucketConsumer) Consume(context.Context, redisstate.BucketRef, ratelimit.Config, int64, time.Time) (ratelimit.Decision, error) {
	if f.err != nil {
		return ratelimit.Decision{}, f.err
	}

	return f.decision, nil
}

type fakeBlockedRequestAuditor struct {
	requests []audit.BlockedRequest
	err      error
}

func (f *fakeBlockedRequestAuditor) LogBlocked(_ context.Context, request audit.BlockedRequest) error {
	if f.err != nil {
		return f.err
	}

	f.requests = append(f.requests, request)
	return nil
}

func TestEnforceRateLimitReturnsServiceUnavailableOnBucketFailure(t *testing.T) {
	apiKeyID := uuid.MustParse("6275915d-bec2-47d2-b8fb-dd0985e76ad8")
	middleware := EnforceRateLimit("ping", 1, fakeEffectivePolicyResolver{
		resolution: policies.Resolution{
			Policy: policies.Policy{
				Capacity:              10,
				RefillTokens:          1,
				RefillIntervalSeconds: 60,
			},
			MatchedScopeType: policies.ScopeGlobal,
		},
		found: true,
	}, fakeBucketConsumer{
		err: redisstate.ErrBucketContention,
	}, nil, time.Now)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/ping", nil)
	request = request.WithContext(auth.WithAPIKey(request.Context(), auth.APIKey{ID: apiKeyID}))

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected next handler not to run")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestDurationToHeaderSecondsRoundsUp(t *testing.T) {
	if got := durationToHeaderSeconds(1500 * time.Millisecond); got != 2 {
		t.Fatalf("expected 2 seconds, got %d", got)
	}
	if got := durationToHeaderSeconds(0); got != 0 {
		t.Fatalf("expected 0 seconds, got %d", got)
	}
}

func TestBucketRefFromResolution(t *testing.T) {
	apiKeyID := uuid.MustParse("1c80c1e9-f96d-4870-8e32-6194470c8531")
	ref := bucketRefFromResolution(policies.Resolution{
		MatchedScopeType:       policies.ScopeAPIKeyRoute,
		MatchedScopeIdentifier: &apiKeyID,
		MatchedRoutePattern:    stringPointer("orders"),
	})

	if ref.ScopeType != policies.ScopeAPIKeyRoute {
		t.Fatalf("expected scope type %q, got %q", policies.ScopeAPIKeyRoute, ref.ScopeType)
	}
	if ref.ScopeIdentifier == nil || *ref.ScopeIdentifier != apiKeyID {
		t.Fatalf("expected scope identifier %s, got %#v", apiKeyID, ref.ScopeIdentifier)
	}
	if ref.RoutePattern == nil || *ref.RoutePattern != "orders" {
		t.Fatalf("expected route pattern orders, got %#v", ref.RoutePattern)
	}
}

func TestEnforceRateLimitReturnsServiceUnavailableOnResolverFailure(t *testing.T) {
	apiKeyID := uuid.MustParse("51aa3b44-bd44-4f3d-a041-30ad0c17b253")
	middleware := EnforceRateLimit("ping", 1, fakeEffectivePolicyResolver{
		err: errors.New("boom"),
	}, fakeBucketConsumer{}, nil, time.Now)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/protected/ping", nil)
	request = request.WithContext(auth.WithAPIKey(request.Context(), auth.APIKey{ID: apiKeyID}))

	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("expected next handler not to run")
	})).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
}
