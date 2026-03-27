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
	Demo     DemoConfig
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

type DemoConfig struct {
	PublicMode bool
	RawAPIKey  string
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

	publicDemoMode, err := boolFromEnv("PUBLIC_DEMO_MODE", false)
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		AppEnv: valueFromEnv("APP_ENV", "development"),
		Server: ServerConfig{
			Addr: serverAddrFromEnv(),
		},
		Admin: AdminConfig{
			Token: valueFromEnv("ADMIN_TOKEN", "dev-admin-token"),
		},
		Demo: DemoConfig{
			PublicMode: publicDemoMode,
			RawAPIKey:  valueFromEnv("PUBLIC_DEMO_RAW_API_KEY", ""),
		},
		Postgres: PostgresConfig{
			DSN: valueFromEnv("POSTGRES_DSN", "postgres://postgres:postgres@postgres:5432/rate_limiter?sslmode=disable"),
		},
		Redis: RedisConfig{
			Addr: valueFromEnv("REDIS_URL", valueFromEnv("REDIS_ADDR", "redis:6379")),
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

func boolFromEnv(key string, fallback bool) (bool, error) {
	value, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a valid boolean: %w", key, err)
	}

	return parsed, nil
}

func serverAddrFromEnv() string {
	if value, ok := os.LookupEnv("SERVER_ADDR"); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}

	if value, ok := os.LookupEnv("PORT"); ok && strings.TrimSpace(value) != "" {
		return ":" + strings.TrimSpace(value)
	}

	return ":8080"
}
