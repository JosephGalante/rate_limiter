package ratelimit

import (
	"errors"
	"time"
)

var (
	ErrInvalidCapacity       = errors.New("capacity must be greater than zero")
	ErrInvalidRefillTokens   = errors.New("refill_tokens must be greater than zero")
	ErrInvalidRefillInterval = errors.New("refill_interval must be greater than zero")
	ErrInvalidRequestCost    = errors.New("request_cost must be greater than zero")
)

type Config struct {
	Capacity       int64
	RefillTokens   int64
	RefillInterval time.Duration
}

type State struct {
	TokensRemaining int64
	LastRefillAt    time.Time
}

type Decision struct {
	Allowed    bool
	Limit      int64
	Remaining  int64
	RetryAfter time.Duration
	ResetAt    time.Time
	BucketTTL  time.Duration
	State      State
}

func Apply(now time.Time, current State, cfg Config, cost int64) (Decision, error) {
	if err := validateConfig(cfg); err != nil {
		return Decision{}, err
	}
	if cost <= 0 {
		return Decision{}, ErrInvalidRequestCost
	}

	now = now.UTC()
	current = normalizeState(current, now, cfg.Capacity)
	refilled := refill(now, current, cfg)

	decision := Decision{
		Limit: cfg.Capacity,
		State: refilled,
	}

	if refilled.TokensRemaining >= cost {
		decision.Allowed = true
		decision.State.TokensRemaining -= cost
	} else {
		decision.RetryAfter = durationUntilTokens(now, decision.State, cfg, cost)
	}

	decision.Remaining = decision.State.TokensRemaining
	decision.ResetAt = now.Add(durationUntilFull(now, decision.State, cfg))
	decision.BucketTTL = bucketTTL(now, decision.State, cfg)

	return decision, nil
}

func validateConfig(cfg Config) error {
	if cfg.Capacity <= 0 {
		return ErrInvalidCapacity
	}
	if cfg.RefillTokens <= 0 {
		return ErrInvalidRefillTokens
	}
	if cfg.RefillInterval <= 0 {
		return ErrInvalidRefillInterval
	}

	return nil
}

func normalizeState(current State, now time.Time, capacity int64) State {
	current.LastRefillAt = current.LastRefillAt.UTC()
	if current.LastRefillAt.IsZero() {
		current.LastRefillAt = now
	}
	if current.TokensRemaining < 0 {
		current.TokensRemaining = 0
	}
	if current.TokensRemaining > capacity {
		current.TokensRemaining = capacity
	}

	return current
}

func refill(now time.Time, current State, cfg Config) State {
	if now.Before(current.LastRefillAt) {
		return current
	}

	elapsed := now.Sub(current.LastRefillAt)
	intervals := elapsed / cfg.RefillInterval
	if intervals <= 0 {
		return current
	}

	refilledTokens := current.TokensRemaining + int64(intervals)*cfg.RefillTokens
	if refilledTokens > cfg.Capacity {
		refilledTokens = cfg.Capacity
	}

	return State{
		TokensRemaining: refilledTokens,
		LastRefillAt:    current.LastRefillAt.Add(intervals * cfg.RefillInterval),
	}
}

func durationUntilTokens(now time.Time, state State, cfg Config, cost int64) time.Duration {
	if state.TokensRemaining >= cost {
		return 0
	}

	tokensNeeded := cost - state.TokensRemaining
	intervalsNeeded := ceilDiv(tokensNeeded, cfg.RefillTokens)

	return durationUntilNextRefill(now, state.LastRefillAt, cfg.RefillInterval) + time.Duration(intervalsNeeded-1)*cfg.RefillInterval
}

func durationUntilFull(now time.Time, state State, cfg Config) time.Duration {
	if state.TokensRemaining >= cfg.Capacity {
		return 0
	}

	tokensNeeded := cfg.Capacity - state.TokensRemaining
	intervalsNeeded := ceilDiv(tokensNeeded, cfg.RefillTokens)

	return durationUntilNextRefill(now, state.LastRefillAt, cfg.RefillInterval) + time.Duration(intervalsNeeded-1)*cfg.RefillInterval
}

func durationUntilNextRefill(now time.Time, lastRefillAt time.Time, interval time.Duration) time.Duration {
	if now.Before(lastRefillAt) {
		return interval
	}

	elapsed := now.Sub(lastRefillAt)
	remainder := elapsed % interval
	if remainder == 0 {
		return interval
	}

	return interval - remainder
}

func bucketTTL(now time.Time, state State, cfg Config) time.Duration {
	untilFull := durationUntilFull(now, state, cfg)
	if untilFull <= 0 {
		return cfg.RefillInterval
	}

	return untilFull + cfg.RefillInterval
}

func ceilDiv(numerator int64, denominator int64) int64 {
	return (numerator + denominator - 1) / denominator
}
