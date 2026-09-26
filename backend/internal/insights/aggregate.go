package insights

import (
	"sort"
	"time"
)

// summarize computes the month total, the average of previous months and uncategorized.
func summarize(rows []spendRow, m time.Time, withData map[time.Time]bool) Summary {
	var s Summary
	totals := map[time.Time]int64{}
	for _, r := range rows {
		totals[r.Month] += r.Total
		if r.Month.Equal(m) && r.CategoryID == 0 {
			s.Uncategorized.Count += r.Count
			s.Uncategorized.Total += r.Total
		}
	}
	s.Total = totals[m]
	var sum int64
	n := 0
	for k := 1; k <= 3; k++ {
		pm := m.AddDate(0, -k, 0)
		if withData[pm] {
			sum += totals[pm]
			n++
		}
	}
	if n > 0 {
		avg := roundDiv(sum, int64(n))
		s.Prev3Avg = &avg
	}
	return s
}

// roundDiv divides rounding half away from zero.
func roundDiv(a, b int64) int64 {
	if (a < 0) != (b < 0) {
		return (a - b/2) / b
	}
	return (a + b/2) / b
}

// byCategory groups the month's categorized spending by category and subcategory, biggest first.
func byCategory(rows []spendRow, m time.Time, cats categories) []CategoryTotal {
	byCat := map[int64]*CategoryTotal{}
	subs := map[int64]map[int64]*SubcategoryTotal{}
	for _, r := range rows {
		if !r.Month.Equal(m) || r.CategoryID == 0 {
			continue
		}
		parent := cats.parentOf(r.CategoryID)
		ct := byCat[parent.ID]
		if ct == nil {
			ct = &CategoryTotal{ID: parent.ID, Name: parent.Name, Color: parent.Color}
			byCat[parent.ID] = ct
			subs[parent.ID] = map[int64]*SubcategoryTotal{}
		}
		ct.Total += r.Total
		ct.Count += r.Count
		st := subs[parent.ID][r.CategoryID]
		if st == nil {
			st = &SubcategoryTotal{ID: r.CategoryID, Name: cats[r.CategoryID].Name}
			subs[parent.ID][r.CategoryID] = st
		}
		st.Total += r.Total
		st.Count += r.Count
	}
	out := make([]CategoryTotal, 0, len(byCat))
	for id, ct := range byCat {
		for _, st := range subs[id] {
			ct.Subcategories = append(ct.Subcategories, *st)
		}
		sort.Slice(ct.Subcategories, func(i, j int) bool {
			a, b := ct.Subcategories[i], ct.Subcategories[j]
			return a.Total > b.Total || (a.Total == b.Total && a.ID < b.ID)
		})
		out = append(out, *ct)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Total > out[j].Total || (out[i].Total == out[j].Total && out[i].ID < out[j].ID)
	})
	return out
}

// evolution returns the 12 months ending at m with totals per category (in category order).
func evolution(rows []spendRow, m time.Time, cats categories) []EvolutionMonth {
	out := make([]EvolutionMonth, 12)
	index := map[time.Time]int{}
	for i := range out {
		mm := m.AddDate(0, i-11, 0)
		out[i].Month = mm
		out[i].Categories = []CategoryAmount{}
		index[mm] = i
	}
	perCat := make([]map[int64]int64, 12)
	for i := range perCat {
		perCat[i] = map[int64]int64{}
	}
	for _, r := range rows {
		i, ok := index[r.Month]
		if !ok {
			continue
		}
		out[i].Total += r.Total
		if r.CategoryID == 0 {
			out[i].Uncategorized += r.Total
			continue
		}
		perCat[i][cats.parentOf(r.CategoryID).ID] += r.Total
	}
	for i := range out {
		for id, total := range perCat[i] {
			c := cats[id]
			out[i].Categories = append(out[i].Categories, CategoryAmount{ID: id, Name: c.Name, Color: c.Color, Total: total})
		}
		sort.Slice(out[i].Categories, func(a, b int) bool {
			ca, cb := cats[out[i].Categories[a].ID], cats[out[i].Categories[b].ID]
			return ca.Position < cb.Position || (ca.Position == cb.Position && ca.ID < cb.ID)
		})
	}
	return out
}

// topMerchants returns the 10 keys with the biggest spending in the month.
func topMerchants(rows []spendRow, m time.Time, keys map[string]keyInfo) []Merchant {
	byKey := map[string]*Merchant{}
	lastDate := map[string]time.Time{}
	for _, r := range rows {
		if !r.Month.Equal(m) {
			continue
		}
		mc := byKey[r.Key]
		if mc == nil {
			mc = &Merchant{Key: r.Key}
			byKey[r.Key] = mc
		}
		mc.Total += r.Total
		mc.Count += r.Count
		if r.LastDate.After(lastDate[r.Key]) || mc.Alias == "" {
			lastDate[r.Key] = r.LastDate
			mc.Alias = r.LastDescription
		}
	}
	out := make([]Merchant, 0, len(byKey))
	for k, mc := range byKey {
		info := keys[k]
		if info.Alias != "" {
			mc.Alias = info.Alias
		}
		mc.CategoryPath = info.CategoryPath
		out = append(out, *mc)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Total > out[j].Total || (out[i].Total == out[j].Total && out[i].Key < out[j].Key)
	})
	if len(out) > 10 {
		out = out[:10]
	}
	return out
}
