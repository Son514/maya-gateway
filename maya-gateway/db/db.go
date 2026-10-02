package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq" // registers the "postgres" driver with database/sql
)

// Config holds the values needed to build a PostgreSQL connection string.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// ConfigFromEnv reads the DB_* environment variables.
func ConfigFromEnv() Config {
	return Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
	}
}

// Connect opens a pool and verifies it is reachable. The returned *sql.DB is a
// connection pool: create it once and share it, never per request.
func Connect(ctx context.Context, cfg Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name,
	)

	pool, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.PingContext(pingCtx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
