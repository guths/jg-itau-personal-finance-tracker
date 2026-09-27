// Package storetest gives integration tests a migrated, freshly seeded MySQL database.
package storetest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"

	"jgfinance/internal/config"
	"jgfinance/internal/store"
)

// tables lists every application table, children first.
var tables = []string{"transactions", "questions", "rules", "documents", "accounts", "categories"}

// New connects to MYSQL_TEST_DSN (from the repository .env or the environment), applies the
// migrations, empties every table and restores the seeded categories and rules. It skips the
// test when the database is unreachable.
//
// Each test package gets its own database (<test db>_<package dir>) so packages can run in
// parallel under `go test ./...`; tests inside one package must not call t.Parallel.
func New(t testing.TB) *store.Store {
	t.Helper()
	dsn, err := packageDSN(testDSN())
	if err != nil {
		t.Fatalf("storetest: %v", err)
	}
	if err := ensureDatabase(dsn); err != nil {
		t.Skipf("storetest: MySQL de teste indisponível (%v); rode `make db-up`", err)
	}
	s, err := store.Open(dsn)
	if err != nil {
		t.Skipf("storetest: MySQL de teste indisponível (%v)", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := reset(s); err != nil {
		t.Fatalf("storetest: limpar banco: %v", err)
	}
	return s
}

// testDSN reads MYSQL_TEST_DSN from the repository's .env files, with the environment winning.
func testDSN() string {
	var files []string
	if root := moduleRoot(); root != "" {
		files = append(files, filepath.Join(root, "..", ".env"), filepath.Join(root, ".env"))
	}
	return config.Load(files...).MySQLTestDSN
}

// moduleRoot walks up from the working directory to the directory holding go.mod.
func moduleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

var nonWord = regexp.MustCompile(`[^a-z0-9_]+`)

// packageDSN points dsn at a database dedicated to the test package being run.
func packageDSN(dsn string) (string, error) {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return "", fmt.Errorf("MYSQL_TEST_DSN inválido: %w", err)
	}
	wd, _ := os.Getwd()
	suffix := nonWord.ReplaceAllString(strings.ToLower(filepath.Base(wd)), "_")
	cfg.DBName = cfg.DBName + "_" + suffix
	return cfg.FormatDSN(), nil
}

// ensureDatabase creates the package database if needed.
func ensureDatabase(dsn string) error {
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil {
		return err
	}
	name := cfg.DBName
	cfg.DBName = ""
	cfg.Timeout = 3 * time.Second
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return err
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err = db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS `"+name+"` CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci")
	return err
}

// reset truncates every table and runs the seed statements again.
func reset(s *store.Store) error {
	ctx := context.Background()
	conn, err := s.DB.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}
	for _, t := range tables {
		if _, err := conn.ExecContext(ctx, "TRUNCATE TABLE "+t); err != nil {
			return err
		}
	}
	if _, err := conn.ExecContext(ctx, "SET FOREIGN_KEY_CHECKS = 1"); err != nil {
		return err
	}
	stmts, err := store.SeedStatements()
	if err != nil {
		return err
	}
	for _, st := range stmts {
		if _, err := conn.ExecContext(ctx, st); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}
	return nil
}
