package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/joe/distributed-rate-limiter/internal/auth"
	"github.com/joe/distributed-rate-limiter/internal/config"
	"github.com/joe/distributed-rate-limiter/internal/db"
	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
	"github.com/joe/distributed-rate-limiter/internal/policies"
	"github.com/joe/distributed-rate-limiter/internal/redisstate"
)

type bootstrapPolicyResult struct {
	Status                string `json:"status"`
	ScopeType             string `json:"scope_type"`
	ScopeIdentifier       string `json:"scope_identifier,omitempty"`
	RoutePattern          string `json:"route_pattern,omitempty"`
	Capacity              int32  `json:"capacity"`
	RefillTokens          int32  `json:"refill_tokens"`
	RefillIntervalSeconds int32  `json:"refill_interval_seconds"`
}

type bootstrapResult struct {
	APIBaseURL  string                  `json:"api_base_url"`
	UIURL       string                  `json:"ui_url"`
	AdminToken  string                  `json:"admin_token"`
	DemoAPIKey  bootstrapAPIKeyResult   `json:"demo_api_key"`
	Policies    []bootstrapPolicyResult `json:"policies"`
	GeneratedAt string                  `json:"generated_at"`
}

type bootstrapAPIKeyResult struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	KeyPrefix string `json:"key_prefix"`
	RawKey    string `json:"raw_key"`
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbPool, err := db.Open(ctx, cfg.Postgres.DSN)
	if err != nil {
		fail(err)
	}
	defer dbPool.Close()

	redisClient, err := redisstate.NewClient(cfg.Redis.Addr, cfg.Redis.DB)
	if err != nil {
		fail(err)
	}
	defer redisClient.Close()

	queries := dbsqlc.New(dbPool)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
	apiKeyCodec := auth.NewAPIKeyCodec(cfg.Security.KeyHashPepper)
	apiKeyService := auth.NewAPIKeyService(
		queries,
		apiKeyCodec,
		redisstate.NewAPIKeyAuthCache(redisClient, cfg.Redis.APIKeyCacheTTL),
		logger,
	)
	policyService := policies.NewService(queries, redisstate.NewPolicyProjectionStore(redisClient))

	createdKey, err := ensureDemoAPIKey(ctx, cfg, queries, apiKeyService, apiKeyCodec)
	if err != nil {
		fail(fmt.Errorf("create demo api key: %w", err))
	}

	results := []bootstrapPolicyResult{
		ensurePolicy(ctx, policyService, policies.CreatePolicyInput{
			ScopeType:             policies.ScopeGlobal,
			Capacity:              12,
			RefillTokens:          4,
			RefillIntervalSeconds: 60,
		}),
		ensurePolicy(ctx, policyService, policies.CreatePolicyInput{
			ScopeType:             policies.ScopeRoute,
			RoutePattern:          stringPointer("report"),
			Capacity:              10,
			RefillTokens:          2,
			RefillIntervalSeconds: 60,
		}),
		ensurePolicy(ctx, policyService, policies.CreatePolicyInput{
			ScopeType:             policies.ScopeAPIKeyRoute,
			ScopeIdentifier:       &createdKey.APIKey.ID,
			RoutePattern:          stringPointer("orders"),
			Capacity:              6,
			RefillTokens:          2,
			RefillIntervalSeconds: 30,
		}),
	}

	payload := bootstrapResult{
		APIBaseURL: "",
		UIURL:      cfg.UI.AllowedOrigin,
		AdminToken: cfg.Admin.Token,
		DemoAPIKey: bootstrapAPIKeyResult{
			ID:        createdKey.APIKey.ID.String(),
			Name:      createdKey.APIKey.Name,
			KeyPrefix: createdKey.APIKey.KeyPrefix,
			RawKey:    createdKey.RawKey,
		},
		Policies:    results,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	payload.APIBaseURL = inferAPIBaseURL(cfg.Server.Addr)

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		fail(fmt.Errorf("encode bootstrap output: %w", err))
	}
}

func ensureDemoAPIKey(
	ctx context.Context,
	cfg config.Config,
	queries *dbsqlc.Queries,
	service *auth.APIKeyService,
	codec *auth.APIKeyCodec,
) (auth.CreatedAPIKey, error) {
	rawKey := strings.TrimSpace(cfg.Demo.RawAPIKey)
	if cfg.Demo.PublicMode && rawKey == "" {
		return auth.CreatedAPIKey{}, fmt.Errorf("PUBLIC_DEMO_RAW_API_KEY must be set when PUBLIC_DEMO_MODE=true")
	}

	if rawKey == "" {
		return service.Create(ctx, auth.CreateAPIKeyInput{
			Name: fmt.Sprintf("demo-%s", time.Now().UTC().Format("20060102-150405")),
		})
	}

	keyHash := codec.Hash(rawKey)
	if _, err := queries.GetAPIKeyByHash(ctx, keyHash); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return auth.CreatedAPIKey{}, fmt.Errorf("get api key by hash: %w", err)
		}

		keyPrefix, prefixErr := codec.Prefix(rawKey)
		if prefixErr != nil {
			return auth.CreatedAPIKey{}, prefixErr
		}

		if _, createErr := queries.CreateAPIKey(ctx, dbsqlc.CreateAPIKeyParams{
			Name:      "public-demo",
			KeyPrefix: keyPrefix,
			KeyHash:   keyHash,
		}); createErr != nil {
			return auth.CreatedAPIKey{}, fmt.Errorf("create stable demo api key: %w", createErr)
		}
	}

	apiKey, err := service.ResolveActiveByRawKey(ctx, rawKey)
	if err != nil {
		return auth.CreatedAPIKey{}, fmt.Errorf("resolve stable demo api key: %w", err)
	}

	return auth.CreatedAPIKey{
		APIKey: apiKey,
		RawKey: rawKey,
	}, nil
}

func ensurePolicy(ctx context.Context, service *policies.Service, input policies.CreatePolicyInput) bootstrapPolicyResult {
	result := bootstrapPolicyResult{
		ScopeType:             input.ScopeType,
		Capacity:              input.Capacity,
		RefillTokens:          input.RefillTokens,
		RefillIntervalSeconds: input.RefillIntervalSeconds,
	}
	if input.ScopeIdentifier != nil {
		result.ScopeIdentifier = input.ScopeIdentifier.String()
	}
	if input.RoutePattern != nil {
		result.RoutePattern = *input.RoutePattern
	}

	_, err := service.Create(ctx, input)
	switch {
	case err == nil:
		result.Status = "created"
	case errors.Is(err, policies.ErrPolicyConflict):
		result.Status = "kept_existing"
	default:
		fail(fmt.Errorf("ensure policy %s/%s: %w", input.ScopeType, result.RoutePattern, err))
	}

	return result
}

func inferAPIBaseURL(serverAddr string) string {
	serverAddr = strings.TrimSpace(serverAddr)
	switch {
	case serverAddr == "", serverAddr == ":8080":
		return "http://localhost:8080"
	case strings.HasPrefix(serverAddr, "http://"), strings.HasPrefix(serverAddr, "https://"):
		return serverAddr
	case strings.HasPrefix(serverAddr, ":"):
		return "http://localhost" + serverAddr
	default:
		return "http://" + serverAddr
	}
}

func stringPointer(value string) *string {
	return &value
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "demo bootstrap failed: %v\n", err)
	os.Exit(1)
}
