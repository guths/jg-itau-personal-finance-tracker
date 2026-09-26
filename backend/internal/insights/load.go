package insights

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

// category is a row of the categories table.
type category struct {
	ID       int64
	ParentID int64 // 0 for top-level categories
	Name     string
	Color    string
	Position int
}

type categories map[int64]category

// parentOf returns the top-level category of a subcategory id (or of itself).
func (c categories) parentOf(id int64) category {
	cat := c[id]
	if cat.ParentID != 0 {
		return c[cat.ParentID]
	}
	return cat
}

func (c categories) path(id int64) string {
	cat, ok := c[id]
	if !ok {
		return ""
	}
	if cat.ParentID == 0 {
		return cat.Name
	}
	return c[cat.ParentID].Name + " › " + cat.Name
}

func loadCategories(ctx context.Context, db *sql.DB) (categories, error) {
	rows, err := db.QueryContext(ctx, `SELECT id, COALESCE(parent_id, 0), name, COALESCE(color, ''), position FROM categories`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := categories{}
	for rows.Next() {
		var c category
		if err := rows.Scan(&c.ID, &c.ParentID, &c.Name, &c.Color, &c.Position); err != nil {
			return nil, err
		}
		out[c.ID] = c
	}
	return out, rows.Err()
}

// spendRow aggregates the spending of one key, subcategory, kind and month.
type spendRow struct {
	Month           time.Time
	Key             string
	CategoryID      int64 // 0 = uncategorized
	Kind            string
	Total           int64
	Count           int
	LastDate        time.Time
	LastDescription string
}

// loadSpending reads expense/refund aggregates for months from..to (inclusive).
func loadSpending(ctx context.Context, db *sql.DB, from, to time.Time) ([]spendRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT reference_month, merchant_key, COALESCE(category_id, 0), kind,
		SUM(amount_cents), COUNT(*), MAX(occurred_on),
		SUBSTRING_INDEX(GROUP_CONCAT(description ORDER BY occurred_on DESC, id DESC SEPARATOR '\n'), '\n', 1)
		FROM transactions
		WHERE kind IN ('expense', 'refund') AND reference_month BETWEEN ? AND ?
		GROUP BY reference_month, merchant_key, category_id, kind`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []spendRow
	for rows.Next() {
		var r spendRow
		if err := rows.Scan(&r.Month, &r.Key, &r.CategoryID, &r.Kind, &r.Total, &r.Count, &r.LastDate, &r.LastDescription); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// keyInfo is what the dashboard shows about a key besides amounts.
type keyInfo struct {
	Alias        string
	Recurring    string // auto | yes | no
	CategoryPath string
}

// keysOf lists the distinct keys of rows from month from on.
func keysOf(rows []spendRow, from time.Time) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		if !r.Month.Before(from) && !seen[r.Key] {
			seen[r.Key] = true
			out = append(out, r.Key)
		}
	}
	return out
}

// loadKeyInfo reads each key's rule (alias, recurring, category) and, without a rule category,
// the category of its most recent categorized transaction.
func loadKeyInfo(ctx context.Context, db *sql.DB, cats categories, keys []string) (map[string]keyInfo, error) {
	out := map[string]keyInfo{}
	if len(keys) == 0 {
		return out, nil
	}
	args := make([]any, len(keys))
	for i, k := range keys {
		args[i] = k
	}
	in := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	rows, err := db.QueryContext(ctx, `SELECT merchant_key, COALESCE(alias, ''), recurring, COALESCE(category_id, 0)
		FROM rules WHERE merchant_key IN (`+in+`)`, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k, alias, rec string
		var cat int64
		if err := rows.Scan(&k, &alias, &rec, &cat); err != nil {
			rows.Close()
			return nil, err
		}
		out[k] = keyInfo{Alias: alias, Recurring: rec, CategoryPath: cats.path(cat)}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = db.QueryContext(ctx, `SELECT t.merchant_key, t.category_id FROM transactions t
		JOIN (SELECT merchant_key, MAX(id) AS id FROM transactions
		      WHERE category_id IS NOT NULL AND merchant_key IN (`+in+`) GROUP BY merchant_key) last ON last.id = t.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var cat int64
		if err := rows.Scan(&k, &cat); err != nil {
			return nil, err
		}
		info := out[k]
		if info.CategoryPath == "" {
			info.CategoryPath = cats.path(cat)
		}
		out[k] = info
	}
	return out, rows.Err()
}
