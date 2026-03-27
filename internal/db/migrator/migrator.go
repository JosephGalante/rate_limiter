package migrator

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	appmigrations "github.com/joe/distributed-rate-limiter/migrations"
)

func Up(ctx context.Context, dsn string) error {
	sourceDriver, err := iofs.New(appmigrations.Files, ".")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}

	db, err := openWithRetry(ctx, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("create postgres migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		if sourceErr != nil || databaseErr != nil {
			// Close errors are non-fatal at shutdown time.
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}

func openWithRetry(ctx context.Context, dsn string) (*sql.DB, error) {
	var lastErr error

	for attempt := 0; attempt < 10; attempt++ {
		connConfig, err := pgx.ParseConfig(dsn)
		if err != nil {
			return nil, fmt.Errorf("parse postgres config: %w", err)
		}

		db := stdlib.OpenDB(*connConfig)
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = db.PingContext(pingCtx)
		cancel()
		if err == nil {
			return db, nil
		}

		lastErr = err
		_ = db.Close()

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("wait for postgres: %w", ctx.Err())
		case <-time.After(2 * time.Second):
		}
	}

	return nil, fmt.Errorf("wait for postgres: %w", lastErr)
}
