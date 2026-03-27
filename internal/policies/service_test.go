package policies

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/joe/distributed-rate-limiter/internal/db"
	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
)

func TestValidateCreateInput(t *testing.T) {
	t.Run("rejects invalid scope shape", func(t *testing.T) {
		_, err := validateCreateInput(CreatePolicyInput{
			ScopeType:             ScopeGlobal,
			RoutePattern:          stringPointer("ping"),
			Capacity:              10,
			RefillTokens:          1,
			RefillIntervalSeconds: 1,
		})
		if err != ErrInvalidScopeShape {
			t.Fatalf("expected ErrInvalidScopeShape, got %v", err)
		}
	})

	t.Run("rejects invalid route pattern", func(t *testing.T) {
		_, err := validateCreateInput(CreatePolicyInput{
			ScopeType:             ScopeRoute,
			RoutePattern:          stringPointer("unknown"),
			Capacity:              10,
			RefillTokens:          1,
			RefillIntervalSeconds: 1,
		})
		if err != ErrInvalidRoutePattern {
			t.Fatalf("expected ErrInvalidRoutePattern, got %v", err)
		}
	})
}

func TestServiceCreateAndList(t *testing.T) {
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

	resetPolicyTables(t, ctx, pool)

	service := NewService(dbsqlc.New(pool), nil)
	created, err := service.Create(ctx, CreatePolicyInput{
		ScopeType:             ScopeGlobal,
		Capacity:              20,
		RefillTokens:          5,
		RefillIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	if created.ScopeType != ScopeGlobal {
		t.Fatalf("expected scope type %q, got %q", ScopeGlobal, created.ScopeType)
	}

	policies, err := service.List(ctx)
	if err != nil {
		t.Fatalf("list policies: %v", err)
	}

	if len(policies) != 1 {
		t.Fatalf("expected 1 policy, got %d", len(policies))
	}
}

func TestServiceRejectsDuplicateActiveGlobalPolicy(t *testing.T) {
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

	resetPolicyTables(t, ctx, pool)

	service := NewService(dbsqlc.New(pool), nil)
	_, err = service.Create(ctx, CreatePolicyInput{
		ScopeType:             ScopeGlobal,
		Capacity:              20,
		RefillTokens:          5,
		RefillIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("seed global policy: %v", err)
	}

	_, err = service.Create(ctx, CreatePolicyInput{
		ScopeType:             ScopeGlobal,
		Capacity:              30,
		RefillTokens:          6,
		RefillIntervalSeconds: 60,
	})
	if err != ErrPolicyConflict {
		t.Fatalf("expected ErrPolicyConflict, got %v", err)
	}
}

func TestServiceUpdate(t *testing.T) {
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

	resetPolicyTables(t, ctx, pool)

	service := NewService(dbsqlc.New(pool), nil)
	created, err := service.Create(ctx, CreatePolicyInput{
		ScopeType:             ScopeGlobal,
		Capacity:              20,
		RefillTokens:          5,
		RefillIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	updated, err := service.Update(ctx, created.ID, UpdatePolicyInput{
		ScopeType:             ScopeRoute,
		RoutePattern:          stringPointer("report"),
		Capacity:              8,
		RefillTokens:          2,
		RefillIntervalSeconds: 30,
	})
	if err != nil {
		t.Fatalf("update policy: %v", err)
	}

	if updated.ScopeType != ScopeRoute {
		t.Fatalf("expected scope type %q, got %q", ScopeRoute, updated.ScopeType)
	}

	if updated.RoutePattern == nil || *updated.RoutePattern != "report" {
		t.Fatalf("expected route pattern report, got %#v", updated.RoutePattern)
	}
}

func TestServiceDeactivate(t *testing.T) {
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

	resetPolicyTables(t, ctx, pool)

	service := NewService(dbsqlc.New(pool), nil)
	created, err := service.Create(ctx, CreatePolicyInput{
		ScopeType:             ScopeGlobal,
		Capacity:              20,
		RefillTokens:          5,
		RefillIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	deactivated, err := service.Deactivate(ctx, created.ID)
	if err != nil {
		t.Fatalf("deactivate policy: %v", err)
	}

	if deactivated.IsActive {
		t.Fatalf("expected policy to be inactive after deactivation")
	}
}

func resetPolicyTables(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	if _, err := pool.Exec(ctx, "TRUNCATE request_audit_logs, rate_limit_policies, api_keys, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func stringPointer(value string) *string {
	return &value
}
