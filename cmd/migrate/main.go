package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/joe/distributed-rate-limiter/internal/config"
	"github.com/joe/distributed-rate-limiter/internal/db/migrator"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := migrator.Up(ctx, cfg.Postgres.DSN); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintf(os.Stderr, "migrate failed: %v\n", err)
	os.Exit(1)
}
