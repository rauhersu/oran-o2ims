/*
SPDX-FileCopyrightText: Red Hat

SPDX-License-Identifier: Apache-2.0
*/

// Package testhelpers provides utilities for integration testing with real databases.
package testhelpers

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps a testcontainers PostgreSQL instance for testing.
type PostgresContainer struct {
	Container *postgres.PostgresContainer
	Pool      *pgxpool.Pool
	ConnStr   string
}

// NewPostgresContainer creates a new PostgreSQL container for integration testing.
// The container uses PostgreSQL 16 Alpine and is configured with a test database.
func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	// Start PostgreSQL container
	pc, err := postgres.Run(ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("resources_test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to start postgres container: %w", err)
	}

	// Get connection string (without SSL for testing)
	connStr, err := pc.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = pc.Terminate(ctx)
		return nil, fmt.Errorf("failed to get connection string: %w", err)
	}

	// Create connection pool
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		_ = pc.Terminate(ctx)
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	// Verify connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		_ = pc.Terminate(ctx)
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresContainer{
		Container: pc,
		Pool:      pool,
		ConnStr:   connStr,
	}, nil
}

// Terminate closes the connection pool and stops the container.
func (pc *PostgresContainer) Terminate(ctx context.Context) error {
	if pc.Pool != nil {
		pc.Pool.Close()
	}
	if pc.Container != nil {
		if err := pc.Container.Terminate(ctx); err != nil {
			return fmt.Errorf("failed to terminate container: %w", err)
		}
	}
	return nil
}

// TableExists checks if a table exists in the database.
func (pc *PostgresContainer) TableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	err := pc.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)
	`, tableName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if table %s exists: %w", tableName, err)
	}
	return exists, nil
}

// ColumnExists checks if a column exists in a table in the database.
func (pc *PostgresContainer) ColumnExists(ctx context.Context, tableName, columnName string) (bool, error) {
	var exists bool
	err := pc.Pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT FROM information_schema.columns
			WHERE table_schema = 'public'
			AND table_name = $1
			AND column_name = $2
		)
	`, tableName, columnName).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if column %s exists in table %s: %w", columnName, tableName, err)
	}
	return exists, nil
}

// GetMigrationVersion returns the current migration version from schema_migrations table.
func (pc *PostgresContainer) GetMigrationVersion(ctx context.Context) (int, bool, error) {
	var version int
	var dirty bool
	err := pc.Pool.QueryRow(ctx, `
		SELECT version, dirty FROM schema_migrations LIMIT 1
	`).Scan(&version, &dirty)
	if err != nil {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}
	return version, dirty, nil
}

// migrateConnStr converts a postgres:// connection string to pgx5:// for golang-migrate.
func migrateConnStr(connStr string) string {
	return strings.Replace(connStr, "postgres://", "pgx5://", 1)
}

// RunMigrations applies database migrations from an embedded filesystem.
// The migrationsFS should contain SQL migration files, and subdir is the directory within the FS.
func (pc *PostgresContainer) RunMigrations(migrationsFS embed.FS, subdir string) error {
	// Create migration source from embedded files
	source, err := iofs.New(migrationsFS, subdir)
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create migrate instance with pgx5 driver
	m, err := migrate.NewWithSourceInstance("iofs", source, migrateConnStr(pc.ConnStr))
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	// Run migrations
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}
