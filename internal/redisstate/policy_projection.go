package redisstate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/joe/distributed-rate-limiter/internal/policies"
)

const (
	policyProjectionPrefix      = "ratelimit:policy"
	policyProjectionDefaultKey  = "default"
	policyProjectionAllRouteKey = "ALL"
)

type PolicyProjectionStore struct {
	client *redis.Client
}

func NewPolicyProjectionStore(client *redis.Client) *PolicyProjectionStore {
	return &PolicyProjectionStore{client: client}
}

func (s *PolicyProjectionStore) SyncPolicy(ctx context.Context, previous *policies.Policy, current policies.Policy) error {
	commands := []func(redis.Pipeliner) error{
		func(pipe redis.Pipeliner) error {
			payload, err := json.Marshal(current)
			if err != nil {
				return err
			}

			pipe.Set(ctx, policyProjectionKey(current), payload, 0)
			return nil
		},
	}

	if previous != nil {
		previousKey := policyProjectionKey(*previous)
		currentKey := policyProjectionKey(current)
		if previousKey != currentKey {
			commands = append(commands, func(pipe redis.Pipeliner) error {
				pipe.Del(ctx, previousKey)
				return nil
			})
		}
	}

	pipe := s.client.TxPipeline()
	for _, command := range commands {
		if err := command(pipe); err != nil {
			return err
		}
	}

	_, err := pipe.Exec(ctx)
	return err
}

func (s *PolicyProjectionStore) RemovePolicy(ctx context.Context, policy policies.Policy) error {
	return s.client.Del(ctx, policyProjectionKey(policy)).Err()
}

func (s *PolicyProjectionStore) ReplacePolicies(ctx context.Context, items []policies.Policy) error {
	keys, err := s.projectionKeys(ctx)
	if err != nil {
		return err
	}

	if len(keys) == 0 && len(items) == 0 {
		return nil
	}

	pipe := s.client.TxPipeline()
	if len(keys) > 0 {
		pipe.Del(ctx, keys...)
	}

	for _, policy := range items {
		payload, err := json.Marshal(policy)
		if err != nil {
			return err
		}

		pipe.Set(ctx, policyProjectionKey(policy), payload, 0)
	}

	_, err = pipe.Exec(ctx)
	return err
}

func (s *PolicyProjectionStore) GetProjectedPolicy(ctx context.Context, scopeType string, scopeIdentifier *uuid.UUID, routePattern *string) (policies.Policy, bool, error) {
	key := policyProjectionKeyFromParts(scopeType, scopeIdentifier, routePattern)
	payload, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return policies.Policy{}, false, nil
		}

		return policies.Policy{}, false, err
	}

	var policy policies.Policy
	if err := json.Unmarshal(payload, &policy); err != nil {
		return policies.Policy{}, false, err
	}

	return policy, true, nil
}

func (s *PolicyProjectionStore) projectionKeys(ctx context.Context) ([]string, error) {
	keys := make([]string, 0)
	var cursor uint64

	for {
		batch, nextCursor, err := s.client.Scan(ctx, cursor, policyProjectionPrefix+":*", 100).Result()
		if err != nil {
			return nil, err
		}

		keys = append(keys, batch...)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

func policyProjectionKey(policy policies.Policy) string {
	return policyProjectionKeyFromParts(policy.ScopeType, policy.ScopeIdentifier, policy.RoutePattern)
}

func policyProjectionKeyFromParts(scopeType string, scopeIdentifier *uuid.UUID, routePattern *string) string {
	return fmt.Sprintf(
		"%s:%s:%s:%s",
		policyProjectionPrefix,
		scopeType,
		projectionScopeIdentifier(scopeIdentifier),
		projectionRoutePattern(routePattern),
	)
}

func projectionScopeIdentifier(scopeIdentifier *uuid.UUID) string {
	if scopeIdentifier == nil {
		return policyProjectionDefaultKey
	}

	return scopeIdentifier.String()
}

func projectionRoutePattern(routePattern *string) string {
	if routePattern == nil {
		return policyProjectionAllRouteKey
	}

	return *routePattern
}
