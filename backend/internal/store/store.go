// Package store owns the MySQL database: connection, embedded goose migrations and seed data.
package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrations embed.FS

// seedFile is the migration that creates the initial taxonomy and seeded rules.
const seedFile = "migrations/00002_seed.sql"

// Store wraps the database handle shared by the other packages.
type Store struct {
	DB *sql.DB
}

// Open connects to dsn, checks the connection and applies pending migrations. The DSN must
// use parseTime=true and loc=UTC (spec §5); both are forced here so dates are never shifted.
func Open(dsn string) (*Store, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: DSN inválido: %w", err)
	}
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	if cfg.Params == nil {
		cfg.Params = map[string]string{}
	}
	cfg.Params["time_zone"] = "'+00:00'"
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: conectar ao MySQL: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{DB: db}, nil
}

// Close closes the database handle.
func (s *Store) Close() error { return s.DB.Close() }

func migrate(ctx context.Context, db *sql.DB) error {
	sub, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectMySQL, db, sub)
	if err != nil {
		return fmt.Errorf("store: migrations: %w", err)
	}
	if _, err := p.Up(ctx); err != nil {
		return fmt.Errorf("store: aplicar migrations: %w", err)
	}
	return nil
}

// SeedStatements returns the SQL statements of the seed migration's Up section, so tests can
// restore the initial taxonomy after truncating the tables.
func SeedStatements() ([]string, error) {
	raw, err := migrations.ReadFile(seedFile)
	if err != nil {
		return nil, err
	}
	text := string(raw)
	_, up, ok := strings.Cut(text, "-- +goose Up")
	if !ok {
		return nil, fmt.Errorf("store: %s sem seção Up", seedFile)
	}
	up, _, _ = strings.Cut(up, "-- +goose Down")
	var out []string
	for _, stmt := range strings.Split(up, ";\n") {
		var lines []string
		for _, l := range strings.Split(stmt, "\n") {
			if t := strings.TrimSpace(l); t != "" && !strings.HasPrefix(t, "--") {
				lines = append(lines, l)
			}
		}
		if len(lines) > 0 {
			out = append(out, strings.TrimSuffix(strings.TrimSpace(strings.Join(lines, "\n")), ";"))
		}
	}
	return out, nil
}
