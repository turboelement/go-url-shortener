package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(dsn string) (*PostgresRepository, error) {
	if err := RunMigrations(dsn); err != nil {
		return nil, err
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("error connection to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("error pinging database: %w", err)
	}

	return &PostgresRepository{db: pool}, nil
}

func (r *PostgresRepository) Save(shortID, originalURL string) (string, error) {
	ctx := context.Background()

	var returnedShortID string
	err := r.db.QueryRow(ctx,
		"INSERT INTO urls (short_id, original_url) VALUES ($1, $2) ON CONFLICT (original_url) DO NOTHING RETURNING short_id",
		shortID, originalURL,
	).Scan(&returnedShortID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			err = r.db.QueryRow(ctx,
				"SELECT short_id FROM urls WHERE original_url = $1",
				originalURL,
			).Scan(&returnedShortID)
			if err != nil {
				return "", fmt.Errorf("cannot find existing url: %w", err)
			}
			return returnedShortID, ErrURLAlreadyExists
		}
		return "", fmt.Errorf("error saving to database: %w", err)
	}

	return returnedShortID, nil
}

func (r *PostgresRepository) Get(shortID string) (string, bool) {
	ctx := context.Background()
	var originalURL string
	err := r.db.QueryRow(ctx,
		"SELECT original_url FROM urls WHERE short_id = $1",
		shortID,
	).Scan(&originalURL)
	if err != nil {
		return "", false
	}
	return originalURL, true
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func (r *PostgresRepository) BatchSave(ctx context.Context, items []BatchEntry) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	for _, item := range items {
		_, err := tx.Exec(ctx,
			"INSERT INTO urls (short_id, original_url) VALUES ($1, $2) ON CONFLICT (short_id) DO NOTHING",
			item.ShortID, item.OriginalURL,
		)
		if err != nil {
			return fmt.Errorf("failed to insert %s: %w", item.ShortID, err)
		}
	}

	return tx.Commit(ctx)
}

func (r *PostgresRepository) Close() {
	r.db.Close()
}

func RunMigrations(dsn string) error {
	m, err := migrate.New(
		"file://migrations",
		dsn,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}
