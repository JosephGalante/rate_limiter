package testutil

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const integrationLockID int64 = 424242

func AcquireIntegrationLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool) func() {
	t.Helper()

	conn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire postgres connection for integration lock: %v", err)
	}

	if _, err := conn.Exec(ctx, "SELECT pg_advisory_lock($1)", integrationLockID); err != nil {
		conn.Release()
		t.Fatalf("acquire integration lock: %v", err)
	}

	return func() {
		t.Helper()

		if _, err := conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", integrationLockID); err != nil {
			conn.Release()
			t.Fatalf("release integration lock: %v", err)
		}

		conn.Release()
	}
}
