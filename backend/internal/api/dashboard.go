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
	d, err := s.insights.Dashboard(r.Context(), month)
	if err != nil {
		writeInternal(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, dashboardJSON(d))
}

type summaryJSON struct {
	Month         Month  `json:"month"`
	Total         int64  `json:"total"`
	Prev3Avg      *int64 `json:"prev3_avg"`
	Fixed         int64  `json:"fixed"`
	Uncategorized struct {
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
	Evolution    []evolutionJSON `json:"evolution"`
	TopMerchants []merchantJSON  `json:"top_merchants"`
}

func dashboardJSON(d insights.Dashboard) dashboardResponse {
	out := dashboardResponse{
		ByCategory: []catTotalJSON{}, Recurring: []recurringJSON{}, Evolution: []evolutionJSON{}, TopMerchants: []merchantJSON{},
	}
	s := d.Summary
	out.Summary = summaryJSON{Month: Month(d.Month), Total: s.Total, Prev3Avg: s.Prev3Avg, Fixed: s.Fixed}
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
