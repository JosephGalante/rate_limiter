package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/joe/distributed-rate-limiter/internal/audit"
	"github.com/joe/distributed-rate-limiter/internal/auth"
	"github.com/joe/distributed-rate-limiter/internal/handlers"
	"github.com/joe/distributed-rate-limiter/internal/policies"
	"github.com/joe/distributed-rate-limiter/internal/ratelimit"
	"github.com/joe/distributed-rate-limiter/internal/redisstate"
)

type EffectivePolicyResolver interface {
	Resolve(ctx context.Context, input policies.ResolveInput) (policies.Resolution, bool, error)
}

type BucketConsumer interface {
	Consume(ctx context.Context, ref redisstate.BucketRef, cfg ratelimit.Config, cost int64, now time.Time) (ratelimit.Decision, error)
}

type BlockedRequestAuditor interface {
	LogBlocked(ctx context.Context, request audit.BlockedRequest) error
}

type Clock func() time.Time

// EnforceRateLimit is a middleware factory that applies rate limiting to HTTP handlers based on API key and route.
//
// How it works:
//
// - The function returns a middleware function, parameterized by:
//   - routeID:      The identifier for the route being protected.
//   - cost:         The "weight" or cost in tokens for each request to this route.
//   - resolver:     Used to determine the rate limit policy for a given API key and route.
//   - buckets:      Used to track and consume tokens from the appropriate rate limiting bucket.
//   - clock:        Provides the current time (used for token bucket calculations); defaults to time.Now if nil.
//
// - The returned middleware wraps an HTTP handler. For every request it:
//
//  1. Extracts the authenticated API key from the request context. If the API key is missing (which should not typically occur if authentication middleware runs before this), it returns a 500 Internal Server Error.
//
//  2. Looks up the effective rate limit policy for this route and API key using resolver.Resolve. If no policy is found or if resolution errors, it returns a 503 Service Unavailable with an appropriate message.
//
//  3. Calls buckets. Consume to attempt to take tokens from the rate limiting bucket. If the bucket is unavailable, responds with 503 Service Unavailable.
//
//  4. Writes rate limit headers (such as X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset) to the response, allowing clients to see their quota status.
//
//  5. If the request exceeds the rate limit, sets the Retry-After header and responds with 429 Too Many Requests.
//
//  6. If allowed, forwards the request to the underlying handler.
//
// This middleware ensures that each request is checked against the caller's current rate limiting policy and status, gracefully handling all failure and limit-exceeded conditions.
func EnforceRateLimit(
	routeID string,
	cost int,
	resolver EffectivePolicyResolver,
	buckets BucketConsumer,
	auditor BlockedRequestAuditor,
	clock Clock,
) func(http.Handler) http.Handler {
	if clock == nil {
		clock = time.Now
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Extract the API key from context, fail if missing
			apiKey, ok := auth.APIKeyFromContext(r.Context())
			if !ok {
				handlers.WriteInternalServerError(w)
				return
			}

			// 2. Resolve the effective rate limit policy for this route and key
			resolution, found, err := resolver.Resolve(r.Context(), policies.ResolveInput{
				APIKeyID: &apiKey.ID,
				RouteID:  routeID,
			})
			if err != nil {
				handlers.WriteServiceUnavailable(w, "rate_limit_resolution_unavailable", "effective policy resolution is currently unavailable")
				return
			}
			if !found {
				handlers.WriteServiceUnavailable(w, "missing_rate_limit_policy", "no active rate limit policy matches this request")
				return
			}

			// 3. Attempt to consume from the token bucket
			decision, err := buckets.Consume(
				r.Context(),
				bucketRefFromResolution(resolution),
				ratelimit.Config{
					Capacity:       int64(resolution.Policy.Capacity),
					RefillTokens:   int64(resolution.Policy.RefillTokens),
					RefillInterval: time.Duration(resolution.Policy.RefillIntervalSeconds) * time.Second,
				},
				int64(cost),
				clock().UTC(),
			)
			if err != nil {
				handlers.WriteServiceUnavailable(w, "rate_limit_unavailable", "rate limit state is currently unavailable")
				return
			}

			// 4. Set rate limit headers on the response
			writeRateLimitHeaders(w, decision)
			if !decision.Allowed {
				writeBlockedAuditLog(auditor, audit.BlockedRequest{
					APIKeyID:        apiKey.ID,
					RouteID:         routeID,
					PolicyID:        resolution.Policy.ID,
					RequestCost:     int32(cost),
					TokensRemaining: int32(decision.Remaining),
				})

				// 5. If rate limit exceeded, set Retry-After and return 429
				w.Header().Set("Retry-After", strconv.FormatInt(durationToHeaderSeconds(decision.RetryAfter), 10))
				handlers.WriteTooManyRequests(w, "rate_limit_exceeded", "rate limit exceeded")
				return
			}

			// 6. Request is allowed; dispatch to next handler
			next.ServeHTTP(w, r)
		})
	}
}

func bucketRefFromResolution(resolution policies.Resolution) redisstate.BucketRef {
	return redisstate.BucketRef{
		ScopeType:       resolution.MatchedScopeType,
		ScopeIdentifier: resolution.MatchedScopeIdentifier,
		RoutePattern:    resolution.MatchedRoutePattern,
	}
}

func writeRateLimitHeaders(w http.ResponseWriter, decision ratelimit.Decision) {
	w.Header().Set("X-RateLimit-Limit", strconv.FormatInt(decision.Limit, 10))
	w.Header().Set("X-RateLimit-Remaining", strconv.FormatInt(decision.Remaining, 10))
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.UTC().Unix(), 10))
}

func durationToHeaderSeconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}

	seconds := value / time.Second
	if value%time.Second != 0 {
		seconds++
	}

	return int64(seconds)
}

func writeBlockedAuditLog(auditor BlockedRequestAuditor, request audit.BlockedRequest) {
	if auditor == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := auditor.LogBlocked(ctx, request); err != nil {
		slog.Default().Warn("failed to write blocked request audit log", slog.String("error", err.Error()))
	}
}
