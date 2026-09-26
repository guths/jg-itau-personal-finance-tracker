package insights

import (
	"sort"
	"time"
)

// Recurring statuses (spec §10).
const (
	StatusOK        = "ok"
	StatusIncreased = "aumentou"
	StatusAbsent    = "ausente"
)

// window is how many months (M included) the automatic detection looks at.
const window = 6

// RecurringItem is a recurring expense in the month.
type RecurringItem struct {
	Key          string
	Alias        string // rule alias or last description
	CategoryPath string
	Total        int64    // total in M (0 when absent)
	Average      int64    // average monthly total of the sequence
	ChangePct    *float64 // vs. the previous month; nil without a previous month
	Months       int      // consecutive months of the sequence
	Status       string
}

// keyMonth is one key's expenses in one month.
type keyMonth struct {
	Total int64
	Count int
}

// keyHistory is a key's monthly expenses (only kind expense).
type keyHistory struct {
	months   map[time.Time]keyMonth
	lastDesc string
	lastDate time.Time
}

// recurring detects the recurring expenses of month m (spec §10).
func recurring(rows []spendRow, m time.Time, keys map[string]keyInfo) []RecurringItem {
	hist := map[string]*keyHistory{}
	for _, r := range rows {
		if r.Kind != "expense" || r.Month.Before(m.AddDate(0, -window, 0)) {
			continue
		}
		h := hist[r.Key]
		if h == nil {
			h = &keyHistory{months: map[time.Time]keyMonth{}}
			hist[r.Key] = h
		}
		km := h.months[r.Month]
		km.Total += r.Total
		km.Count += r.Count
		h.months[r.Month] = km
		if !r.Month.After(m) && r.LastDate.After(h.lastDate) {
			h.lastDate, h.lastDesc = r.LastDate, r.LastDescription
		}
	}
	var out []RecurringItem
	prev := m.AddDate(0, -1, 0)
	for key, h := range hist {
		info := keys[key]
		if info.Recurring == "no" {
			continue
		}
		forced := info.Recurring == "yes"
		item := RecurringItem{Key: key, Alias: h.lastDesc, CategoryPath: info.CategoryPath}
		if info.Alias != "" {
			item.Alias = info.Alias
		}
		cur, inM := h.months[m]
		if n, avg, ok := sequence(h, m, forced); ok && inM {
			item.Total, item.Average, item.Months, item.Status = cur.Total, avg, n, StatusOK
			if p, ok := h.months[prev]; ok && p.Total > 0 {
				pct := float64(cur.Total-p.Total) * 100 / float64(p.Total)
				item.ChangePct = &pct
				if cur.Total*10 > p.Total*11 {
					item.Status = StatusIncreased
				}
			}
			out = append(out, item)
			continue
		}
		if !inM {
			if n, avg, ok := sequence(h, prev, forced); ok {
				pct := -100.0
				item.Average, item.Months, item.Status, item.ChangePct = avg, n, StatusAbsent, &pct
				out = append(out, item)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i].Status == StatusAbsent) != (out[j].Status == StatusAbsent) {
			return out[j].Status == StatusAbsent
		}
		return out[i].Total > out[j].Total || (out[i].Total == out[j].Total && out[i].Key < out[j].Key)
	})
	return out
}

// sequence finds the longest run of consecutive months ending at end (at most window months)
// in which the key qualifies as recurring: every month has 1–2 expenses and a total within
// ±20% of the run's median, with at least 2 months. With forced (rule recurring = yes) any
// month with expenses qualifies and a single month is enough. It returns the run length and
// its average monthly total.
func sequence(h *keyHistory, end time.Time, forced bool) (months int, avg int64, ok bool) {
	var totals []int64
	for k := 0; k < window; k++ {
		km, present := h.months[end.AddDate(0, -k, 0)]
		if !present || km.Count == 0 || (!forced && km.Count > 2) {
			break
		}
		totals = append(totals, km.Total)
	}
	if forced {
		if len(totals) == 0 {
			return 0, 0, false
		}
		return len(totals), average(totals), true
	}
	for n := len(totals); n >= 2; n-- {
		if withinMedian(totals[:n]) {
			return n, average(totals[:n]), true
		}
	}
	return 0, 0, false
}

// withinMedian reports whether every total is within ±20% of the median of totals.
func withinMedian(totals []int64) bool {
	s := append([]int64(nil), totals...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	// Twice the median keeps the arithmetic in integers for even lengths.
	med2 := s[len(s)/2] * 2
	if len(s)%2 == 0 {
		med2 = s[len(s)/2-1] + s[len(s)/2]
	}
	if med2 <= 0 {
		return false
	}
	for _, t := range totals {
		diff := 2*t - med2
		if diff < 0 {
			diff = -diff
		}
		if diff*5 > med2 { // |t - median| > 20% of median
			return false
		}
	}
	return true
}

func average(v []int64) int64 {
	var sum int64
	for _, x := range v {
		sum += x
	}
	return roundDiv(sum, int64(len(v)))
}
