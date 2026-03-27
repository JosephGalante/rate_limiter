package redisstate

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/joe/distributed-rate-limiter/internal/ratelimit"
)

const (
	bucketPrefix                 = "ratelimit:bucket"
	bucketFieldTokensRemaining   = "tokens_remaining"
	bucketFieldLastRefillUnixMs  = "last_refill_unix_ms"
	bucketDefaultScopeIdentifier = "default"
	bucketAllRouteKey            = "ALL"
	summaryMetricsKey            = "ratelimit:metrics:summary"
	summaryAllowedField          = "allowed_requests"
	summaryBlockedField          = "blocked_requests"
	defaultBucketMaxRetries      = 32
)

var ErrBucketContention = errors.New("bucket update contention exceeded retry budget")

type BucketRef struct {
	ScopeType       string
	ScopeIdentifier *uuid.UUID
	RoutePattern    *string
}

type BucketSnapshot struct {
	Key              string
	TokensRemaining  int64
	LastRefillUnixMs int64
}

type SummaryMetrics struct {
	AllowedRequests int64
	BlockedRequests int64
}

type BucketStore struct {
	client     *redis.Client
	maxRetries int
}

func NewBucketStore(client *redis.Client) *BucketStore {
	return &BucketStore{
		client:     client,
		maxRetries: defaultBucketMaxRetries,
	}
}

func (s *BucketStore) Consume(ctx context.Context, ref BucketRef, cfg ratelimit.Config, cost int64, now time.Time) (ratelimit.Decision, error) {
	key := bucketKey(ref)
	now = now.UTC()

	var decision ratelimit.Decision
	for attempt := 0; attempt < s.maxRetries; attempt++ {
		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			state, err := readBucketState(ctx, tx, key, cfg, now)
			if err != nil {
				return err
			}

			decision, err = ratelimit.Apply(now, state, cfg, cost)
			if err != nil {
				return err
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.HSet(ctx, key, map[string]any{
					bucketFieldTokensRemaining:  decision.State.TokensRemaining,
					bucketFieldLastRefillUnixMs: decision.State.LastRefillAt.UnixMilli(),
				})
				pipe.Expire(ctx, key, decision.BucketTTL)
				pipe.HIncrBy(ctx, summaryMetricsKey, summaryField(decision.Allowed), 1)
				return nil
			})
			return err
		}, key)
		if err == nil {
			return decision, nil
		}
		if errors.Is(err, redis.TxFailedErr) {
			continue
		}

		return ratelimit.Decision{}, fmt.Errorf("consume bucket %s: %w", key, err)
	}

	return ratelimit.Decision{}, ErrBucketContention
}

func (s *BucketStore) GetBucketSnapshot(ctx context.Context, ref BucketRef) (BucketSnapshot, bool, error) {
	key := bucketKey(ref)
	values, err := s.client.HGetAll(ctx, key).Result()
	if err != nil {
		return BucketSnapshot{}, false, err
	}
	if len(values) == 0 {
		return BucketSnapshot{}, false, nil
	}

	tokensRemaining, err := strconv.ParseInt(values[bucketFieldTokensRemaining], 10, 64)
	if err != nil {
		return BucketSnapshot{}, false, fmt.Errorf("parse tokens_remaining for %s: %w", key, err)
	}
	lastRefillUnixMs, err := strconv.ParseInt(values[bucketFieldLastRefillUnixMs], 10, 64)
	if err != nil {
		return BucketSnapshot{}, false, fmt.Errorf("parse last_refill_unix_ms for %s: %w", key, err)
	}

	return BucketSnapshot{
		Key:              key,
		TokensRemaining:  tokensRemaining,
		LastRefillUnixMs: lastRefillUnixMs,
	}, true, nil
}

func BucketKey(ref BucketRef) string {
	return bucketKey(ref)
}

func (s *BucketStore) GetSummaryMetrics(ctx context.Context) (SummaryMetrics, error) {
	values, err := s.client.HGetAll(ctx, summaryMetricsKey).Result()
	if err != nil {
		return SummaryMetrics{}, err
	}

	return SummaryMetrics{
		AllowedRequests: parseIntOrZero(values[summaryAllowedField]),
		BlockedRequests: parseIntOrZero(values[summaryBlockedField]),
	}, nil
}

func readBucketState(ctx context.Context, tx *redis.Tx, key string, cfg ratelimit.Config, now time.Time) (ratelimit.State, error) {
	values, err := tx.HGetAll(ctx, key).Result()
	if err != nil {
		return ratelimit.State{}, err
	}
	if len(values) == 0 {
		return ratelimit.State{
			TokensRemaining: cfg.Capacity,
			LastRefillAt:    now,
		}, nil
	}

	tokensRemaining, err := strconv.ParseInt(values[bucketFieldTokensRemaining], 10, 64)
	if err != nil {
		return ratelimit.State{}, fmt.Errorf("parse tokens_remaining for %s: %w", key, err)
	}
	lastRefillUnixMs, err := strconv.ParseInt(values[bucketFieldLastRefillUnixMs], 10, 64)
	if err != nil {
		return ratelimit.State{}, fmt.Errorf("parse last_refill_unix_ms for %s: %w", key, err)
	}

	return ratelimit.State{
		TokensRemaining: tokensRemaining,
		LastRefillAt:    time.UnixMilli(lastRefillUnixMs).UTC(),
	}, nil
}

func bucketKey(ref BucketRef) string {
	return fmt.Sprintf(
		"%s:%s:%s:%s",
		bucketPrefix,
		ref.ScopeType,
		bucketScopeIdentifier(ref.ScopeIdentifier),
		bucketRouteKey(ref.RoutePattern),
	)
}

func bucketScopeIdentifier(scopeIdentifier *uuid.UUID) string {
	if scopeIdentifier == nil {
		return bucketDefaultScopeIdentifier
	}

	return scopeIdentifier.String()
}

func bucketRouteKey(routePattern *string) string {
	if routePattern == nil {
		return bucketAllRouteKey
	}

	return *routePattern
}

func summaryField(allowed bool) string {
	if allowed {
		return summaryAllowedField
	}

	return summaryBlockedField
}

func parseIntOrZero(raw string) int64 {
	if raw == "" {
		return 0
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}

	return value
}
