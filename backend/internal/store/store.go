// Package store owns all database access.
//
// Right now it only wires up the pgx connection pool. Once we add SQL queries
// under queries/ and run `sqlc generate`, type-safe query methods will be
// generated INTO this same package (files like users.sql.go). Never hand-edit
// those generated files.
package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Connect opens a Postgres connection pool and verifies it with a ping.
//
// A *pgxpool.Pool is safe for concurrent use by many goroutines (HTTP
// requests), so we create exactly one and share it across the app.
func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
