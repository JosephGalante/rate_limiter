package redisstate

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/joe/distributed-rate-limiter/internal/ratelimit"
)

func TestBucketStoreConsumeAndMetrics(t *testing.T) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("REDIS_ADDR is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer client.Close()

	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}

	store := NewBucketStore(client)
	now := time.Unix(1_700_000_000, 0).UTC()
	ref := BucketRef{ScopeType: "global"}
	cfg := ratelimit.Config{
		Capacity:       5,
		RefillTokens:   2,
		RefillInterval: 10 * time.Second,
	}

	decision, err := store.Consume(ctx, ref, cfg, 3, now)
	if err != nil {
		t.Fatalf("consume allowed: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("expected first request to be allowed")
	}
	if decision.Remaining != 2 {
		t.Fatalf("expected 2 tokens remaining, got %d", decision.Remaining)
	}

	decision, err = store.Consume(ctx, ref, cfg, 4, now.Add(5*time.Second))
	if err != nil {
		t.Fatalf("consume blocked: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected second request to be blocked")
	}
	if decision.RetryAfter != 5*time.Second {
		t.Fatalf("expected retry after 5s, got %s", decision.RetryAfter)
	}

	snapshot, found, err := store.GetBucketSnapshot(ctx, ref)
	if err != nil {
		t.Fatalf("get bucket snapshot: %v", err)
	}
	if !found {
		t.Fatalf("expected bucket snapshot to exist")
	}
	if snapshot.TokensRemaining != 2 {
		t.Fatalf("expected stored tokens remaining 2, got %d", snapshot.TokensRemaining)
	}

	metrics, err := store.GetSummaryMetrics(ctx)
	if err != nil {
		t.Fatalf("get summary metrics: %v", err)
	}
	if metrics.AllowedRequests != 1 {
		t.Fatalf("expected 1 allowed request, got %d", metrics.AllowedRequests)
	}
	if metrics.BlockedRequests != 1 {
		t.Fatalf("expected 1 blocked request, got %d", metrics.BlockedRequests)
	}
}

func TestBucketStoreConsumeConcurrentRequests(t *testing.T) {
	redisAddr := os.Getenv("REDIS_ADDR")
	if redisAddr == "" {
		t.Skip("REDIS_ADDR is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer client.Close()

	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}

	store := NewBucketStore(client)
	now := time.Unix(1_700_000_000, 0).UTC()
	ref := BucketRef{ScopeType: "global"}
	cfg := ratelimit.Config{
		Capacity:       5,
		RefillTokens:   1,
		RefillInterval: time.Minute,
	}

	const requests = 20
	var wg sync.WaitGroup
	wg.Add(requests)

	results := make(chan bool, requests)
	errorsCh := make(chan error, requests)

	for i := 0; i < requests; i++ {
		go func() {
			defer wg.Done()

			decision, err := store.Consume(ctx, ref, cfg, 1, now)
			if err != nil {
				errorsCh <- err
				return
			}

			results <- decision.Allowed
		}()
	}

	wg.Wait()
	close(results)
	close(errorsCh)

	for err := range errorsCh {
		t.Fatalf("unexpected consume error: %v", err)
	}

	var allowed int
	var blocked int
	for result := range results {
		if result {
			allowed++
			continue
		}

		blocked++
	}

	if allowed != 5 {
		t.Fatalf("expected 5 allowed requests, got %d", allowed)
	}
	if blocked != 15 {
		t.Fatalf("expected 15 blocked requests, got %d", blocked)
	}

	snapshot, found, err := store.GetBucketSnapshot(ctx, ref)
	if err != nil {
		t.Fatalf("get bucket snapshot: %v", err)
	}
	if !found {
		t.Fatalf("expected bucket snapshot to exist")
	}
	if snapshot.TokensRemaining != 0 {
		t.Fatalf("expected stored tokens remaining 0, got %d", snapshot.TokensRemaining)
	}

	metrics, err := store.GetSummaryMetrics(ctx)
	if err != nil {
		t.Fatalf("get summary metrics: %v", err)
	}
	if metrics.AllowedRequests != 5 {
		t.Fatalf("expected 5 allowed requests, got %d", metrics.AllowedRequests)
	}
	if metrics.BlockedRequests != 15 {
		t.Fatalf("expected 15 blocked requests, got %d", metrics.BlockedRequests)
	}
}
