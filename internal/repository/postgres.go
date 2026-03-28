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
	if err := runMigrations(dsn); err != nil {
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

func (r *PostgresRepository) Save(ctx context.Context, shortID, originalURL string) (string, error) {
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

func (r *PostgresRepository) Get(ctx context.Context, shortID string) (string, error) {
	var originalURL string
	var isDeleted bool
	err := r.db.QueryRow(ctx,
		"SELECT original_url, is_deleted FROM urls WHERE short_id = $1",
		shortID,
	).Scan(&originalURL, &isDeleted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrURLNotFound
		}
		return "", err
	}
	if isDeleted {
		return "", ErrURLMarkedAsDeleted
	}
	return originalURL, nil
}

func (r *PostgresRepository) Ping(ctx context.Context) error {
	return r.db.Ping(ctx)
}

func (r *PostgresRepository) SaveWithUser(ctx context.Context, shortID, originalURL, userID string) (string, error) {
	var returnedShortID string
	err := r.db.QueryRow(ctx,
		"INSERT INTO urls (short_id, original_url, user_id) VALUES ($1, $2, $3) ON CONFLICT (original_url) DO NOTHING RETURNING short_id",
		shortID, originalURL, userID,
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

func (r *PostgresRepository) GetUserURLs(ctx context.Context, userID string) ([]UserURL, error) {
	rows, err := r.db.Query(ctx,
		"SELECT short_id, original_url FROM urls WHERE user_id = $1 AND is_deleted = false ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("error querying user urls: %w", err)
	}
	defer rows.Close()

	var results []UserURL
	for rows.Next() {
		var shortID, originalURL string
		if err := rows.Scan(&shortID, &originalURL); err != nil {
			return nil, fmt.Errorf("error scanning row: %w", err)
		}
		results = append(results, UserURL{
			ShortURL:    shortID,
			OriginalURL: originalURL,
		})
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}

func (r *PostgresRepository) BatchSave(ctx context.Context, userID string, items []BatchEntry) error {
	if len(items) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, item := range items {
		if userID != "" {
			batch.Queue(
				"INSERT INTO urls (short_id, original_url, user_id) VALUES ($1, $2, $3) ON CONFLICT (short_id) DO NOTHING",
				item.ShortID, item.OriginalURL, userID,
			)
		} else {
			batch.Queue(
				"INSERT INTO urls (short_id, original_url) VALUES ($1, $2) ON CONFLICT (short_id) DO NOTHING",
				item.ShortID, item.OriginalURL,
			)
		}
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range items {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to send batch: %w", err)
		}
	}

	return nil
}

func (r *PostgresRepository) DeleteUserURLs(ctx context.Context, userID string, shortIDs []string) error {
	if len(shortIDs) == 0 {
		return nil
	}

	batch := &pgx.Batch{}

	for _, shortID := range shortIDs {
		batch.Queue(
			"UPDATE urls SET is_deleted = true WHERE short_id = $1 AND user_id = $2",
			shortID, userID,
		)
	}

	br := r.db.SendBatch(ctx, batch)
	defer br.Close()

	for range shortIDs {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to delete url: %w", err)
		}
	}

	return nil
}

func (r *PostgresRepository) Close() {
	r.db.Close()
}

func runMigrations(dsn string) error {
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
