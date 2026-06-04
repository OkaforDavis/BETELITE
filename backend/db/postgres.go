package db

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

// Connect initializes the PostgreSQL connection pool
func Connect(ctx context.Context, databaseURL string) error {
	if databaseURL == "" {
		log.Println("[WARN] DATABASE_URL is empty. Running without database connection.")
		Pool = nil
		return nil
	}

	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return err
	}

	// Configure pool settings here if necessary
	config.MaxConns = 20

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return err
	}

	if err := pool.Ping(ctx); err != nil {
		return err
	}

	Pool = pool
	log.Println("[INFO] Successfully connected to PostgreSQL")
	return nil
}

// Close closes the connection pool
func Close() {
	if Pool != nil {
		Pool.Close()
		log.Println("[INFO] PostgreSQL connection pool closed")
	}
}
