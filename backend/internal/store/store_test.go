package store_test

import (
	"context"
	"strings"
	"testing"

	"jgfinance/internal/store"
	"jgfinance/internal/store/storetest"
)

func TestSeedStatements(t *testing.T) {
	stmts, err := store.SeedStatements()
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 5 {
		t.Fatalf("got %d statements, want 5", len(stmts))
	}
	for _, s := range stmts {
		if !strings.HasPrefix(s, "INSERT INTO") || strings.HasSuffix(s, ";") {
			t.Errorf("bad statement: %q", s)
		}
	}
}

func TestMigrationsAndSeed(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()

	var cats, subs, rules int
	must(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id IS NULL AND color IS NOT NULL`).Scan(&cats))
	must(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM categories WHERE parent_id IS NOT NULL AND color IS NULL`).Scan(&subs))
	must(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM rules WHERE source = 'user' AND mode = 'always'`).Scan(&rules))
	if cats != 13 || subs != 55 || rules != 3 {
		t.Errorf("categories=%d subcategories=%d rules=%d, want 13/55/3", cats, subs, rules)
	}

	var path string
	must(t, s.DB.QueryRowContext(ctx, `SELECT CONCAT(c.name, ' › ', s.name) FROM rules r
		JOIN categories s ON s.id = r.category_id JOIN categories c ON c.id = s.parent_id
		WHERE r.merchant_key = 'REPASSE DE IOF EM R$'`).Scan(&path))
	if path != "Taxas › IOF" {
		t.Errorf("IOF rule -> %q", path)
	}
	var colors int
	must(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(DISTINCT color) FROM categories WHERE parent_id IS NULL`).Scan(&colors))
	if colors != 13 {
		t.Errorf("distinct colors = %d, want 13", colors)
	}
	var tables int
	must(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE()
		AND table_name IN ('accounts','documents','categories','rules','questions','transactions','installments')`).Scan(&tables))
	if tables != 7 {
		t.Errorf("tables = %d, want 7", tables)
	}
}

func TestResetRemovesDataAndReseeds(t *testing.T) {
	s := storetest.New(t)
	ctx := context.Background()
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO accounts (provider, kind, external_id, name) VALUES ('itau','checking','1/2','x')`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO rules (merchant_key, source) VALUES ('X', 'llm')`); err != nil {
		t.Fatal(err)
	}
	s = storetest.New(t)
	var accounts, rules int
	must(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM accounts`).Scan(&accounts))
	must(t, s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM rules`).Scan(&rules))
	if accounts != 0 || rules != 3 {
		t.Errorf("after reset: accounts=%d rules=%d", accounts, rules)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
