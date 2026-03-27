package redisstate

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/joe/distributed-rate-limiter/internal/db"
	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
	"github.com/joe/distributed-rate-limiter/internal/policies"
	"github.com/joe/distributed-rate-limiter/internal/testutil"
)

func TestPolicyProjectionLifecycleAndRebuild(t *testing.T) {
	dsn := os.Getenv("POSTGRES_DSN")
	redisAddr := os.Getenv("REDIS_ADDR")
	if dsn == "" || redisAddr == "" {
		t.Skip("POSTGRES_DSN and REDIS_ADDR must be set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer pool.Close()
	defer testutil.AcquireIntegrationLock(t, ctx, pool)()

	if _, err := pool.Exec(ctx, "TRUNCATE request_audit_logs, rate_limit_policies, api_keys, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	client := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer client.Close()

	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis: %v", err)
	}

	store := NewPolicyProjectionStore(client)
	service := policies.NewService(dbsqlc.New(pool), store)

	created, err := service.Create(ctx, policies.CreatePolicyInput{
		ScopeType:             policies.ScopeGlobal,
		Capacity:              20,
		RefillTokens:          5,
		RefillIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}

	projected, found, err := store.GetProjectedPolicy(ctx, policies.ScopeGlobal, nil, nil)
	if err != nil {
		t.Fatalf("get projected global policy: %v", err)
	}
	if !found {
		t.Fatalf("expected projected global policy to exist")
	}
	if projected.ID != created.ID {
		t.Fatalf("expected projected id %s, got %s", created.ID, projected.ID)
	}

	updated, err := service.Update(ctx, created.ID, policies.UpdatePolicyInput{
		ScopeType:             policies.ScopeRoute,
		RoutePattern:          stringPointer("report"),
		Capacity:              8,
		RefillTokens:          2,
		RefillIntervalSeconds: 30,
	})
	if err != nil {
		t.Fatalf("update policy: %v", err)
	}

	if _, found, err := store.GetProjectedPolicy(ctx, policies.ScopeGlobal, nil, nil); err != nil {
		t.Fatalf("check removed global projection: %v", err)
	} else if found {
		t.Fatalf("expected global projection to be removed after update")
	}

	projected, found, err = store.GetProjectedPolicy(ctx, policies.ScopeRoute, nil, stringPointer("report"))
	if err != nil {
		t.Fatalf("get projected route policy: %v", err)
	}
	if !found {
		t.Fatalf("expected route projection to exist")
	}
	if projected.ID != updated.ID {
		t.Fatalf("expected projected id %s, got %s", updated.ID, projected.ID)
	}

	if _, err := service.Deactivate(ctx, updated.ID); err != nil {
		t.Fatalf("deactivate policy: %v", err)
	}

	if _, found, err := store.GetProjectedPolicy(ctx, policies.ScopeRoute, nil, stringPointer("report")); err != nil {
		t.Fatalf("check removed route projection: %v", err)
	} else if found {
		t.Fatalf("expected route projection to be removed after deactivation")
	}

	second, err := service.Create(ctx, policies.CreatePolicyInput{
		ScopeType:             policies.ScopeGlobal,
		Capacity:              15,
		RefillTokens:          3,
		RefillIntervalSeconds: 45,
	})
	if err != nil {
		t.Fatalf("create second policy: %v", err)
	}

	if err := client.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("flush redis before rebuild: %v", err)
	}

	if err := service.RebuildProjection(ctx); err != nil {
		t.Fatalf("rebuild projection: %v", err)
	}

	projected, found, err = store.GetProjectedPolicy(ctx, policies.ScopeGlobal, nil, nil)
	if err != nil {
		t.Fatalf("get projected policy after rebuild: %v", err)
	}
	if !found {
		t.Fatalf("expected projected policy after rebuild")
	}
	if projected.ID != second.ID {
		t.Fatalf("expected rebuilt projected id %s, got %s", second.ID, projected.ID)
	}
}

func stringPointer(value string) *string {
	return &value
}
