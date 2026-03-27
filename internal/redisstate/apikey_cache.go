package redisstate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/joe/distributed-rate-limiter/internal/auth"
)

type APIKeyAuthCache struct {
	client *redis.Client
	ttl    time.Duration
}

func NewAPIKeyAuthCache(client *redis.Client, ttl time.Duration) *APIKeyAuthCache {
	return &APIKeyAuthCache{
		client: client,
		ttl:    ttl,
	}
}

func (c *APIKeyAuthCache) GetByHash(ctx context.Context, keyHash string) (auth.APIKey, bool, error) {
	payload, err := c.client.Get(ctx, authCacheKey(keyHash)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return auth.APIKey{}, false, nil
		}

		return auth.APIKey{}, false, err
	}

	var apiKey auth.APIKey
	if err := json.Unmarshal(payload, &apiKey); err != nil {
		return auth.APIKey{}, false, err
	}

	return apiKey, true, nil
}

func (c *APIKeyAuthCache) SetByHash(ctx context.Context, keyHash string, apiKey auth.APIKey) error {
	payload, err := json.Marshal(apiKey)
	if err != nil {
		return err
	}

	return c.client.Set(ctx, authCacheKey(keyHash), payload, c.ttl).Err()
}

func (c *APIKeyAuthCache) DeleteByHash(ctx context.Context, keyHash string) error {
	return c.client.Del(ctx, authCacheKey(keyHash)).Err()
}

func authCacheKey(keyHash string) string {
	return fmt.Sprintf("apikey:by_hash:%s", keyHash)
}
