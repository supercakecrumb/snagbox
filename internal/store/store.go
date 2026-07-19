// Package store owns the Postgres connection pool and schema migrations.
package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/supercakecrumb/snagbox/migrations"
)

// Store wraps the pgx connection pool shared by all repositories.
type Store struct {
	Pool *pgxpool.Pool
}

// Open connects a pgxpool to databaseURL, pings it, and applies goose
// migrations (embedded) via the pgx stdlib driver.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	sqlDB := stdlib.OpenDBFromPool(pool)
	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("postgres"); err != nil {
		_ = sqlDB.Close()
		pool.Close()
		return nil, fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		_ = sqlDB.Close()
		pool.Close()
		return nil, fmt.Errorf("apply migrations: %w", err)
	}
	// Closing the stdlib wrapper does not close the underlying pool.
	if err := sqlDB.Close(); err != nil {
		pool.Close()
		return nil, fmt.Errorf("close migration connection: %w", err)
	}

	return &Store{Pool: pool}, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	s.Pool.Close()
}
