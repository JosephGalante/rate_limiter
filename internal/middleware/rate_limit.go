package middleware

import (
	"context"
	"net/http"
	"strconv"
	"time"

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

type Clock func() time.Time

func EnforceRateLimit(routeID string, cost int, resolver EffectivePolicyResolver, buckets BucketConsumer, clock Clock) func(http.Handler) http.Handler {
	if clock == nil {
		clock = time.Now
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey, ok := auth.APIKeyFromContext(r.Context())
			if !ok {
				handlers.WriteInternalServerError(w)
				return
			}

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

			decision, err := buckets.Consume(r.Context(), bucketRefFromResolution(resolution), ratelimit.Config{
				Capacity:       int64(resolution.Policy.Capacity),
				RefillTokens:   int64(resolution.Policy.RefillTokens),
				RefillInterval: time.Duration(resolution.Policy.RefillIntervalSeconds) * time.Second,
			}, int64(cost), clock().UTC())
			if err != nil {
				handlers.WriteServiceUnavailable(w, "rate_limit_unavailable", "rate limit state is currently unavailable")
				return
			}

			writeRateLimitHeaders(w, decision)
			if !decision.Allowed {
				w.Header().Set("Retry-After", strconv.FormatInt(durationToHeaderSeconds(decision.RetryAfter), 10))
				handlers.WriteTooManyRequests(w, "rate_limit_exceeded", "rate limit exceeded")
				return
			}

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
