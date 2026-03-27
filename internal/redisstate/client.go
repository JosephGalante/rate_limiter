package redisstate

import (
	"strings"

	"github.com/redis/go-redis/v9"
)

func NewClient(addr string, db int) (*redis.Client, error) {
	if strings.HasPrefix(addr, "redis://") || strings.HasPrefix(addr, "rediss://") {
		options, err := redis.ParseURL(addr)
		if err != nil {
			return nil, err
		}
		if db != 0 {
			options.DB = db
		}

		return redis.NewClient(options), nil
	}

	return redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   db,
	}), nil
}
