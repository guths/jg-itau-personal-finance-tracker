package api

import (
	"net/http"
	"time"

	"jgfinance/internal/insights"
)

func (s *Server) getMonths(w http.ResponseWriter, r *http.Request) {
	months, err := s.insights.Months(r.Context())
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	out := make([]Month, len(months))
	for i, m := range months {
		out[i] = Month(m)
	}
	writeJSON(w, http.StatusOK, out)
}

// getDashboard serves ?month=AAAA-MM; without month, the most recent month with data (or the
// current month when there is none).
func (s *Server) getDashboard(w http.ResponseWriter, r *http.Request) {
	now := s.now().UTC()
	var month time.Time
	if v := r.URL.Query().Get("month"); v != "" {
		m, err := parseMonth(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		month = m
	} else {
		months, err := s.insights.Months(r.Context())
		if err != nil {
			writeInternal(w, r, err)
			return
		}
		month = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		if len(months) > 0 {
			month = months[0]
		}
	}
	today := localToday(now)
	d, err := s.insights.Dashboard(r.Context(), month, today)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboardJSON(d))
}

// localToday is today's civil date in the machine's time zone, as midnight UTC.
func localToday(now time.Time) time.Time {
	l := now.In(time.Local)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, time.UTC)
}

type summaryJSON struct {
	Month              Month  `json:"month"`
	Total              int64  `json:"total"`
	Prev3Avg           *int64 `json:"prev3_avg"`
	Fixed              int64  `json:"fixed"`
	OpenBills          int64  `json:"open_bills"`
	FutureInstallments int64  `json:"future_installments"`
	Uncategorized      struct {
		Count int   `json:"count"`
		Total int64 `json:"total"`
	} `json:"uncategorized"`
}

type subTotalJSON struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Total int64  `json:"total"`
	Count int    `json:"count"`
}

type catTotalJSON struct {
	ID            int64          `json:"id"`
	Name          string         `json:"name"`
	Color         string         `json:"color"`
	Total         int64          `json:"total"`
	Count         int            `json:"count"`
	Subcategories []subTotalJSON `json:"subcategories"`
}

type recurringJSON struct {
	Key          string   `json:"key"`
	Alias        string   `json:"alias"`
	CategoryPath string   `json:"category_path"`
	Total        int64    `json:"total"`
	Average      int64    `json:"average"`
	ChangePct    *float64 `json:"change_pct"`
	Months       int      `json:"months"`
	Status       string   `json:"status"`
}

type billJSON struct {
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`
	DueDate     Date   `json:"due_date"`
	Total       int64  `json:"total"`
}

type accountAmountJSON struct {
	AccountID   int64  `json:"account_id"`
	AccountName string `json:"account_name"`
	Total       int64  `json:"total"`
}

type installmentMonthJSON struct {
	Month     Month               `json:"month"`
	Total     int64               `json:"total"`
	ByAccount []accountAmountJSON `json:"by_account"`
}

type purchaseJSON struct {
	AccountID         int64  `json:"account_id"`
	AccountName       string `json:"account_name"`
	Key               string `json:"key"`
	Description       string `json:"description"`
	PurchaseDate      *Date  `json:"purchase_date"`
	InstallmentAmount int64  `json:"installment_amount"`
	Remaining         int    `json:"remaining"`
	InstallmentTotal  int    `json:"installment_total"`
	RemainingTotal    int64  `json:"remaining_total"`
	NextMonth         Month  `json:"next_month"`
}

type upcomingJSON struct {
	Bills             []billJSON             `json:"bills"`
	ByMonth           []installmentMonthJSON `json:"by_month"`
	Purchases         []purchaseJSON         `json:"purchases"`
	BillsTotal        int64                  `json:"bills_total"`
	InstallmentsTotal int64                  `json:"installments_total"`
	Total             int64                  `json:"total"`
}

type catAmountJSON struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Total int64  `json:"total"`
}

type evolutionJSON struct {
	Month         Month           `json:"month"`
	Total         int64           `json:"total"`
	Uncategorized int64           `json:"uncategorized"`
	Categories    []catAmountJSON `json:"categories"`
}

type merchantJSON struct {
	Key          string `json:"key"`
	Alias        string `json:"alias"`
	Total        int64  `json:"total"`
	Count        int    `json:"count"`
	CategoryPath string `json:"category_path"`
}

type dashboardResponse struct {
	Summary      summaryJSON     `json:"summary"`
	ByCategory   []catTotalJSON  `json:"by_category"`
	Recurring    []recurringJSON `json:"recurring"`
	Upcoming     upcomingJSON    `json:"upcoming"`
	Evolution    []evolutionJSON `json:"evolution"`
	TopMerchants []merchantJSON  `json:"top_merchants"`
}

func dashboardJSON(d insights.Dashboard) dashboardResponse {
	out := dashboardResponse{
		ByCategory: []catTotalJSON{}, Recurring: []recurringJSON{}, Evolution: []evolutionJSON{}, TopMerchants: []merchantJSON{},
	}
	s := d.Summary
	out.Summary = summaryJSON{Month: Month(d.Month), Total: s.Total, Prev3Avg: s.Prev3Avg, Fixed: s.Fixed,
		OpenBills: s.OpenBills, FutureInstallments: s.FutureInstallments}
	out.Summary.Uncategorized.Count, out.Summary.Uncategorized.Total = s.Uncategorized.Count, s.Uncategorized.Total
	for _, c := range d.ByCategory {
		cj := catTotalJSON{ID: c.ID, Name: c.Name, Color: c.Color, Total: c.Total, Count: c.Count, Subcategories: []subTotalJSON{}}
		for _, sc := range c.Subcategories {
			cj.Subcategories = append(cj.Subcategories, subTotalJSON(sc))
		}
		out.ByCategory = append(out.ByCategory, cj)
	}
	for _, r := range d.Recurring {
		out.Recurring = append(out.Recurring, recurringJSON(r))
	}
	u := d.Upcoming
	out.Upcoming = upcomingJSON{Bills: []billJSON{}, ByMonth: []installmentMonthJSON{}, Purchases: []purchaseJSON{},
		BillsTotal: u.BillsTotal, InstallmentsTotal: u.InstallmentsTotal, Total: u.Total}
	for _, b := range u.Bills {
		out.Upcoming.Bills = append(out.Upcoming.Bills, billJSON{AccountID: b.AccountID, AccountName: b.AccountName,
			DueDate: Date(b.DueDate), Total: b.TotalCents})
	}
	for _, m := range u.ByMonth {
		mj := installmentMonthJSON{Month: Month(m.Month), Total: m.Total, ByAccount: []accountAmountJSON{}}
		for _, a := range m.ByAccount {
			mj.ByAccount = append(mj.ByAccount, accountAmountJSON(a))
		}
		out.Upcoming.ByMonth = append(out.Upcoming.ByMonth, mj)
	}
	for _, p := range u.Purchases {
		pj := purchaseJSON{AccountID: p.AccountID, AccountName: p.AccountName, Key: p.Key, Description: p.Description,
			InstallmentAmount: p.InstallmentAmount, Remaining: p.Remaining, InstallmentTotal: p.InstallmentTotal,
			RemainingTotal: p.RemainingTotal, NextMonth: Month(p.NextMonth)}
		if p.PurchaseDate != nil {
			d := Date(*p.PurchaseDate)
			pj.PurchaseDate = &d
		}
		out.Upcoming.Purchases = append(out.Upcoming.Purchases, pj)
	}
	for _, e := range d.Evolution {
		ej := evolutionJSON{Month: Month(e.Month), Total: e.Total, Uncategorized: e.Uncategorized, Categories: []catAmountJSON{}}
		for _, c := range e.Categories {
			ej.Categories = append(ej.Categories, catAmountJSON(c))
		}
		out.Evolution = append(out.Evolution, ej)
	}
	for _, m := range d.TopMerchants {
		out.TopMerchants = append(out.TopMerchants, merchantJSON(m))
	}
	return out
}
