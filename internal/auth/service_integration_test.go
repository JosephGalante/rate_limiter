package auth

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joe/distributed-rate-limiter/internal/db"
	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
)

func TestAPIKeyServiceCreateResolveListDeactivate(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_DSN is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer pool.Close()

	resetAPIKeyTables(t, ctx, pool)

	service := NewAPIKeyService(
		dbsqlc.New(pool),
		NewAPIKeyCodec("integration-test-pepper"),
		nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	created, err := service.Create(ctx, CreateAPIKeyInput{Name: "primary"})
	if err != nil {
		t.Fatalf("create api key: %v", err)
	}

	if created.RawKey == "" {
		t.Fatalf("expected raw key to be returned once")
	}

	resolved, err := service.ResolveActiveByRawKey(ctx, created.RawKey)
	if err != nil {
		t.Fatalf("resolve raw api key: %v", err)
	}

	if resolved.ID != created.APIKey.ID {
		t.Fatalf("expected resolved id %s, got %s", created.APIKey.ID, resolved.ID)
	}

	apiKeys, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list api keys: %v", err)
	}

	if len(apiKeys) != 1 {
		t.Fatalf("expected 1 api key, got %d", len(apiKeys))
	}

	if apiKeys[0].Name != "primary" {
		t.Fatalf("expected api key name primary, got %q", apiKeys[0].Name)
	}

	if _, err := service.Deactivate(ctx, created.APIKey.ID); err != nil {
		t.Fatalf("deactivate api key: %v", err)
	}

	if _, err := service.ResolveActiveByRawKey(ctx, created.RawKey); err != ErrAPIKeyNotFound {
		t.Fatalf("expected ErrAPIKeyNotFound after deactivation, got %v", err)
	}
}

func resetAPIKeyTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, "TRUNCATE request_audit_logs, rate_limit_policies, api_keys, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}
