// Package insights computes the dashboard (spec §10 and §12): monthly totals by category,
// recurring expenses, what is still to be paid, evolution and top merchants. Only expense and
// refund transactions count as spending; every amount is in cents.
package insights

import (
	"context"
	"time"

	"jgfinance/internal/store"
)

// Service answers dashboard queries.
type Service struct {
	store *store.Store
}

// New returns a Service.
func New(st *store.Store) *Service { return &Service{store: st} }

// Dashboard is everything the dashboard shows for one month.
type Dashboard struct {
	Month        time.Time
	Summary      Summary
	ByCategory   []CategoryTotal
	Recurring    []RecurringItem
	Upcoming     Upcoming
	Evolution    []EvolutionMonth
	TopMerchants []Merchant
}

// Summary holds the headline numbers of the month.
type Summary struct {
	Total              int64
	Prev3Avg           *int64 // average of the months with data among M-3..M-1; nil if none
	Fixed              int64  // sum of the month's recurring expenses
	OpenBills          int64  // invoices due today or later
	FutureInstallments int64  // projected installments from the current month on
	Uncategorized      Uncategorized
}

// Uncategorized counts the month's spending without category.
type Uncategorized struct {
	Count int
	Total int64
}

// CategoryTotal is a category's spending in the month with its subcategories.
type CategoryTotal struct {
	ID            int64
	Name          string
	Color         string
	Total         int64
	Count         int
	Subcategories []SubcategoryTotal
}

// SubcategoryTotal is a subcategory's spending in the month.
type SubcategoryTotal struct {
	ID    int64
	Name  string
	Total int64
	Count int
}

// EvolutionMonth is one month of the 12-month evolution chart.
type EvolutionMonth struct {
	Month         time.Time
	Total         int64
	Uncategorized int64
	Categories    []CategoryAmount
}

// CategoryAmount is a category's total within an evolution month.
type CategoryAmount struct {
	ID    int64
	Name  string
	Color string
	Total int64
}

// Merchant is one of the month's biggest keys.
type Merchant struct {
	Key          string
	Alias        string
	Total        int64
	Count        int
	CategoryPath string
}

// Dashboard computes the dashboard of month (any day of it); today anchors "falta pagar".
func (s *Service) Dashboard(ctx context.Context, month, today time.Time) (Dashboard, error) {
	m := monthOf(month)
	d := Dashboard{Month: m}
	cats, err := loadCategories(ctx, s.store.DB)
	if err != nil {
		return d, err
	}
	rows, err := loadSpending(ctx, s.store.DB, m.AddDate(0, -11, 0), m)
	if err != nil {
		return d, err
	}
	withData, err := s.monthsWithData(ctx, m.AddDate(0, -3, 0), m.AddDate(0, -1, 0))
	if err != nil {
		return d, err
	}
	keys, err := loadKeyInfo(ctx, s.store.DB, cats, keysOf(rows, m.AddDate(0, -6, 0)))
	if err != nil {
		return d, err
	}

	d.Summary = summarize(rows, m, withData)
	d.ByCategory = byCategory(rows, m, cats)
	d.Evolution = evolution(rows, m, cats)
	d.TopMerchants = topMerchants(rows, m, keys)
	d.Recurring = recurring(rows, m, keys)
	for _, r := range d.Recurring {
		if r.Status != StatusAbsent {
			d.Summary.Fixed += r.Total
		}
	}
	if d.Upcoming, err = s.upcoming(ctx, today); err != nil {
		return d, err
	}
	d.Summary.OpenBills = d.Upcoming.BillsTotal
	d.Summary.FutureInstallments = d.Upcoming.InstallmentsTotal
	return d, nil
}

// Months lists the reference months that have transactions, most recent first.
func (s *Service) Months(ctx context.Context) ([]time.Time, error) {
	rows, err := s.store.DB.QueryContext(ctx, `SELECT DISTINCT reference_month FROM transactions ORDER BY reference_month DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []time.Time{}
	for rows.Next() {
		var m time.Time
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Service) monthsWithData(ctx context.Context, from, to time.Time) (map[time.Time]bool, error) {
	rows, err := s.store.DB.QueryContext(ctx, `SELECT DISTINCT reference_month FROM transactions
		WHERE reference_month BETWEEN ? AND ?`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[time.Time]bool{}
	for rows.Next() {
		var m time.Time
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out[m] = true
	}
	return out, rows.Err()
}

// monthOf returns the first day of t's month (UTC).
func monthOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}
