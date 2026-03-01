package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(dsn string) (*PostgresRepository, error) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("Error connection to DB: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("Error pinging DB: %w", err)
	}

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS urls (
			short_id VARCHAR(10) PRIMARY KEY,
			original_url TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("Error creating table: %w", err)
	}

	return &PostgresRepository{db: pool}, nil
}

func (r *PostgresRepository) Save(shortID, originalURL string) {
	ctx := context.Background()
	_, err := r.db.Exec(ctx,
		"INSERT INTO urls (short_id, original_url) VALUES ($1, $2) ON CONFLICT (short_id) DO NOTHING",
		shortID, originalURL,
	)
	if err != nil {
		fmt.Printf("Error saving to DB: %v\n", err)
	}
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

func (r *PostgresRepository) Close() {
	r.db.Close()
}
