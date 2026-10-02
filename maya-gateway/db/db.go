package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	// registers the "postgres" driver with database/sql and golang-migrate
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
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

// dsn returns a key/value connection string for lib/pq.
func (c Config) dsn() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.Host, c.Port, c.User, c.Password, c.Name,
	)
}

// url returns a connection URL for golang-migrate, which takes a URL rather
// than a key/value string.
func (c Config) url() string {
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   c.Host + ":" + c.Port,
		Path:   c.Name,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()
	return u.String()
}

// Connect opens a pool and verifies it is reachable. The returned *sql.DB is a
// connection pool: create it once and share it, never per request.
func Connect(ctx context.Context, cfg Config) (*sql.DB, error) {
	pool, err := sql.Open("postgres", cfg.dsn())
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

// MigrateUp applies every pending migration in path. It is safe to call on each
// boot: golang-migrate records applied versions in schema_migrations.
func MigrateUp(cfg Config, path string) error {
	m, err := migrate.New("file://"+path, cfg.url())
	if err != nil {
		return err
	}
	defer m.Close()

	// ErrNoChange means the database is already at the latest version.
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}

	return nil
}

// RequireTable fails if the named table is missing after migrations have run.
// golang-migrate trusts its own version ledger, so a table dropped by hand is
// invisible to it: Up() reports no change and the schema silently drifts.
func RequireTable(ctx context.Context, pool *sql.DB, table string) error {
	var name *string
	if err := pool.QueryRowContext(ctx,
		`SELECT to_regclass($1)::text`, table,
	).Scan(&name); err != nil {
		return err
	}

	if name == nil {
		return fmt.Errorf("table %q is missing after migrations", table)
	}

	return nil
}
