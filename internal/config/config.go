package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv   string
	Server   ServerConfig
	Admin    AdminConfig
	Postgres PostgresConfig
	Redis    RedisConfig
	Security SecurityConfig
	UI       UIConfig
}

type ServerConfig struct {
	Addr string
}

type AdminConfig struct {
	Token string
}

type PostgresConfig struct {
	DSN string
}

type RedisConfig struct {
	Addr           string
	DB             int
	APIKeyCacheTTL time.Duration
}

type SecurityConfig struct {
	KeyHashPepper string
}

type UIConfig struct {
	AllowedOrigin string
}

func Load() (Config, error) {
	redisDB, err := intFromEnv("REDIS_DB", 0)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: valueFromEnv("APP_ENV", "development"),
		Server: ServerConfig{
			Addr: valueFromEnv("SERVER_ADDR", ":8080"),
		},
		Admin: AdminConfig{
			Token: valueFromEnv("ADMIN_TOKEN", "dev-admin-token"),
		},
		Postgres: PostgresConfig{
			DSN: valueFromEnv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/rate_limiter?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr: valueFromEnv("REDIS_ADDR", "redis:6379"),
			DB:   redisDB,
		},
		Security: SecurityConfig{
			KeyHashPepper: valueFromEnv("KEY_HASH_PEPPER", "dev-key-pepper"),
		},
		UI: UIConfig{
			AllowedOrigin: valueFromEnv("CORS_ALLOWED_ORIGIN", "http://localhost:5173"),
		},
	}

	apiKeyCacheTTL, err := durationFromEnv("REDIS_API_KEY_CACHE_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}
	cfg.Redis.APIKeyCacheTTL = apiKeyCacheTTL

	if strings.TrimSpace(cfg.Admin.Token) == "" {
		return Config{}, fmt.Errorf("ADMIN_TOKEN must not be empty")
	}

	if strings.TrimSpace(cfg.Security.KeyHashPepper) == "" {
		return Config{}, fmt.Errorf("KEY_HASH_PEPPER must not be empty")
	}

	return cfg, nil
}

func valueFromEnv(key string, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && strings.TrimSpace(value) != "" {
		return value
	}

	return fallback
}

func intFromEnv(key string, fallback int) (int, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}

	return parsed, nil
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a valid duration: %w", key, err)
	}

	return parsed, nil
}
