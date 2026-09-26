package categorize

import (
	"context"
	"database/sql"
	"sort"
)

// Subcategory is an active subcategory with its display path "Categoria › Subcategoria".
type Subcategory struct {
	ID   int64
	Path string
}

// taxonomy is the set of active subcategories.
type taxonomy struct {
	list []Subcategory
	byID map[int64]string
}

func (t taxonomy) valid(id int64) bool { _, ok := t.byID[id]; return ok }

func (t taxonomy) ids() []int64 {
	out := make([]int64, len(t.list))
	for i, s := range t.list {
		out[i] = s.ID
	}
	return out
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// loadTaxonomy reads the active (non-archived) subcategories of active categories.
func loadTaxonomy(ctx context.Context, q querier) (taxonomy, error) {
	rows, err := q.QueryContext(ctx, `SELECT s.id, CONCAT(c.name, ' › ', s.name) FROM categories s
		JOIN categories c ON c.id = s.parent_id
		WHERE NOT s.archived AND NOT c.archived
		ORDER BY c.position, c.id, s.position, s.id`)
	if err != nil {
		return taxonomy{}, err
	}
	defer rows.Close()
	t := taxonomy{byID: map[int64]string{}}
	for rows.Next() {
		var s Subcategory
		if err := rows.Scan(&s.ID, &s.Path); err != nil {
			return taxonomy{}, err
		}
		t.list = append(t.list, s)
		t.byID[s.ID] = s.Path
	}
	return t, rows.Err()
}

// validIDs keeps the ids that exist in t, without duplicates, preserving order, at most max.
func (t taxonomy) validIDs(ids []int64, max int) []int64 {
	var out []int64
	seen := map[int64]bool{}
	for _, id := range ids {
		if t.valid(id) && !seen[id] && len(out) < max {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
