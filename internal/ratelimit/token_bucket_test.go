package ratelimit

import (
	"errors"
	"testing"
	"time"
)

func TestApplyAllowsAndConsumesTokens(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	decision, err := Apply(now, State{
		TokensRemaining: 10,
		LastRefillAt:    now,
	}, Config{
		Capacity:       10,
		RefillTokens:   2,
		RefillInterval: 10 * time.Second,
	}, 3)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if !decision.Allowed {
		t.Fatalf("expected request to be allowed")
	}
	if decision.Remaining != 7 {
		t.Fatalf("expected 7 tokens remaining, got %d", decision.Remaining)
	}
	if decision.RetryAfter != 0 {
		t.Fatalf("expected zero retry after, got %s", decision.RetryAfter)
	}
	if decision.ResetAt.Sub(now) != 20*time.Second {
		t.Fatalf("expected reset in 20s, got %s", decision.ResetAt.Sub(now))
	}
}

func TestApplyRefillsAndClampsToCapacity(t *testing.T) {
	start := time.Unix(1_700_000_000, 0).UTC()
	now := start.Add(31 * time.Second)

	decision, err := Apply(now, State{
		TokensRemaining: 3,
		LastRefillAt:    start,
	}, Config{
		Capacity:       8,
		RefillTokens:   2,
		RefillInterval: 10 * time.Second,
	}, 1)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if !decision.Allowed {
		t.Fatalf("expected request to be allowed")
	}
	if decision.Remaining != 7 {
		t.Fatalf("expected 7 tokens remaining after refill and cost, got %d", decision.Remaining)
	}
	expectedRefillAt := start.Add(30 * time.Second)
	if !decision.State.LastRefillAt.Equal(expectedRefillAt) {
		t.Fatalf("expected last refill %s, got %s", expectedRefillAt, decision.State.LastRefillAt)
	}
}

func TestApplyBlocksAndReturnsRetryAfter(t *testing.T) {
	start := time.Unix(1_700_000_000, 0).UTC()
	now := start.Add(5 * time.Second)

	decision, err := Apply(now, State{
		TokensRemaining: 1,
		LastRefillAt:    start,
	}, Config{
		Capacity:       10,
		RefillTokens:   2,
		RefillInterval: 10 * time.Second,
	}, 5)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	if decision.Allowed {
		t.Fatalf("expected request to be blocked")
	}
	if decision.Remaining != 1 {
		t.Fatalf("expected remaining tokens to stay at 1, got %d", decision.Remaining)
	}
	if decision.RetryAfter != 15*time.Second {
		t.Fatalf("expected retry after 15s, got %s", decision.RetryAfter)
	}
	if decision.ResetAt.Sub(now) != 45*time.Second {
		t.Fatalf("expected full reset in 45s, got %s", decision.ResetAt.Sub(now))
	}
}

func TestApplyRejectsInvalidInputs(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()

	_, err := Apply(now, State{}, Config{
		Capacity:       0,
		RefillTokens:   1,
		RefillInterval: time.Second,
	}, 1)
	if !errors.Is(err, ErrInvalidCapacity) {
		t.Fatalf("expected ErrInvalidCapacity, got %v", err)
	}

	_, err = Apply(now, State{}, Config{
		Capacity:       1,
		RefillTokens:   1,
		RefillInterval: time.Second,
	}, 0)
	if !errors.Is(err, ErrInvalidRequestCost) {
		t.Fatalf("expected ErrInvalidRequestCost, got %v", err)
	}
}
