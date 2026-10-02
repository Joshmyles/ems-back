package application

import (
	"fmt"
	"math"
	"sort"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

func boolPtr(v bool) *bool { return &v }

func countPtr(n int64) *float64 {
	v := float64(n)
	return &v
}

// percentPtr returns part/total as a percentage, or nil when undefined.
func percentPtr(part, total int64) *float64 {
	if total <= 0 {
		return nil
	}
	v := 100 * float64(part) / float64(total)
	return &v
}

// BuildKPIs turns the overview facts into headline tiles. Previous-period
// values are omitted for all-time reports, which have nothing to compare to.
func BuildKPIs(f analyticsdomain.OverviewFacts, trend []analyticsdomain.TrendPoint, allTime bool) []analyticsdomain.KPI {
	prev := func(v *float64) *float64 {
		if allTime {
			return nil
		}
		return v
	}
	spark := func(fn func(p analyticsdomain.TrendPoint) *float64) []float64 {
		out := make([]float64, 0, len(trend))
		last := 0.0
		for _, p := range trend {
			if v := fn(p); v != nil {
				last = *v
			}
			out = append(out, last)
		}
		return out
	}
	var fuelPerDispatch, prevFuelPerDispatch *float64
	if f.Dispatches > 0 {
		v := f.FuelCost / float64(f.Dispatches)
		fuelPerDispatch = &v
	}
	if f.PrevDispatches > 0 {
		v := f.PrevFuelCost / float64(f.PrevDispatches)
		prevFuelPerDispatch = &v
	}

	return []analyticsdomain.KPI{
		{
			Key: "incidents", Label: "Incidents", Format: "count",
			Value: countPtr(f.Incidents), Previous: prev(countPtr(f.PrevIncidents)),
			Hint:      "Reported in the period",
			Sparkline: spark(func(p analyticsdomain.TrendPoint) *float64 { return countPtr(p.Total) }),
		},
		{
			Key: "critical_share", Label: "Critical (red) share", Format: "percent",
			Value: percentPtr(f.Red, f.Prioritised), Previous: prev(percentPtr(f.PrevRed, f.PrevPrioritised)),
			Hint: "Of prioritised incidents",
			Sparkline: spark(func(p analyticsdomain.TrendPoint) *float64 {
				return percentPtr(p.Red, p.Red+p.Orange+p.Green)
			}),
		},
		{
			Key: "median_dispatch", Label: "Median time to dispatch", Format: "minutes", UpIsGood: boolPtr(false),
			Value: f.MedianDispatch, Previous: prev(f.PrevMedianDispatch),
			Hint:      "Report → ambulance assigned",
			Sparkline: spark(func(p analyticsdomain.TrendPoint) *float64 { return p.MedianDispatch }),
		},
		{
			Key: "dispatch_within_30", Label: "Dispatched within 30 min", Format: "percent", UpIsGood: boolPtr(true),
			Value: percentPtr(f.AssignedWithin30, f.Assigned), Previous: prev(percentPtr(f.PrevAssignedWithin, f.PrevAssigned)),
			Hint: "Of dispatched incidents",
			Sparkline: spark(func(p analyticsdomain.TrendPoint) *float64 {
				return percentPtr(p.AssignedWithin30, p.Assigned)
			}),
		},
		{
			Key: "completion_rate", Label: "Completion rate", Format: "percent", UpIsGood: boolPtr(true),
			Value: percentPtr(f.Completed, f.Closable), Previous: prev(percentPtr(f.PrevCompleted, f.PrevClosable)),
			Hint: "Completed of non-cancelled cases",
			Sparkline: spark(func(p analyticsdomain.TrendPoint) *float64 {
				return percentPtr(p.Completed, p.Total)
			}),
		},
		{
			Key: "transfers", Label: "Inter-facility transfers", Format: "count",
			Value: countPtr(f.Referrals), Previous: prev(countPtr(f.PrevReferrals)),
			Hint:      "Referring → receiving facility",
			Sparkline: spark(func(p analyticsdomain.TrendPoint) *float64 { return countPtr(p.Referrals) }),
		},
		{
			Key: "feedback_rate", Label: "Outcome feedback", Format: "percent", UpIsGood: boolPtr(true),
			Value: percentPtr(f.WithFeedback, f.Completed), Previous: prev(percentPtr(f.PrevWithFeedback, f.PrevCompleted)),
			Hint: "Completed cases with a patient outcome",
		},
		{
			Key: "fuel_per_dispatch", Label: "Fuel cost per dispatch", Format: "currency", UpIsGood: boolPtr(false),
			Value: fuelPerDispatch, Previous: prev(prevFuelPerDispatch),
			Hint:      "UGX of fuel per ambulance dispatch",
			Sparkline: spark(func(p analyticsdomain.TrendPoint) *float64 { v := p.FuelCost; return &v }),
		},
	}
}

func pctOf(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return 100 * float64(part) / float64(total)
}

// change returns the relative change from prev to cur, or false when the
// previous value is too small to compare meaningfully.
func change(cur, prev float64) (float64, bool) {
	if prev <= 0 {
		return 0, false
	}
	return (cur - prev) / prev, true
}

func hourLabel(h int) string { return fmt.Sprintf("%02d:00", h) }

// BuildInsights applies plain rules to the overview facts and returns the
// observations worth a manager's attention, most severe first.
func BuildInsights(f analyticsdomain.OverviewFacts, allTime bool) []analyticsdomain.Insight {
	out := []analyticsdomain.Insight{}
	add := func(key, severity, area, title, detail string) {
		out = append(out, analyticsdomain.Insight{Key: key, Severity: severity, Area: area, Title: title, Detail: detail})
	}
	if f.Incidents == 0 {
		add("no_data", "info", "Volume", "No incidents in this period",
			"Widen the date range or clear the district filter to see activity.")
		return out
	}

	if !allTime && f.PrevIncidents >= 10 {
		if c, ok := change(float64(f.Incidents), float64(f.PrevIncidents)); ok && math.Abs(c) >= 0.15 {
			dir := "up"
			if c < 0 {
				dir = "down"
			}
			add("volume_change", "info", "Volume",
				fmt.Sprintf("Incident volume %s %.0f%% on the previous period", dir, math.Abs(c)*100),
				fmt.Sprintf("%d incidents vs %d in the preceding period of equal length.", f.Incidents, f.PrevIncidents))
		}
	}

	if f.MedianDispatch != nil {
		if !allTime && f.PrevMedianDispatch != nil && f.PrevAssigned >= 10 {
			if c, ok := change(*f.MedianDispatch, *f.PrevMedianDispatch); ok && math.Abs(c) >= 0.2 {
				if c > 0 {
					add("dispatch_slower", "warning", "Response",
						fmt.Sprintf("Dispatch is %.0f%% slower than the previous period", c*100),
						fmt.Sprintf("Median report-to-assignment time rose from %.0f to %.0f minutes.", *f.PrevMedianDispatch, *f.MedianDispatch))
				} else {
					add("dispatch_faster", "good", "Response",
						fmt.Sprintf("Dispatch is %.0f%% faster than the previous period", -c*100),
						fmt.Sprintf("Median report-to-assignment time fell from %.0f to %.0f minutes.", *f.PrevMedianDispatch, *f.MedianDispatch))
				}
			}
		}
		if within := pctOf(f.AssignedWithin30, f.Assigned); f.Assigned >= 10 && within < 50 {
			add("dispatch_sla", "warning", "Response",
				fmt.Sprintf("Only %.0f%% of incidents were dispatched within 30 minutes", within),
				fmt.Sprintf("Median time to dispatch is %.0f minutes. Review the slowest cases on the Response tab.", *f.MedianDispatch))
		}
	}

	if f.Completed >= 10 {
		if rate := pctOf(f.WithFeedback, f.Completed); rate < 20 {
			add("feedback_gap", "critical", "Outcomes",
				fmt.Sprintf("Outcome loop is broken: %.0f%% of completed cases have patient-outcome feedback", rate),
				fmt.Sprintf("%d of %d completed cases have feedback from the receiving facility. Without it, clinical impact cannot be measured.", f.WithFeedback, f.Completed))
		}
	}

	if f.OpenStale > 0 {
		add("stale_open", "warning", "Workflow",
			fmt.Sprintf("%d incidents have been open for more than 24 hours", f.OpenStale),
			"Close finished cases or record why they are still active so live boards stay accurate.")
	}

	if f.ActiveAmbulances >= 2 && f.TopAmbulanceShare >= 0.3 {
		add("fleet_concentration", "warning", "Fleet",
			fmt.Sprintf("%s handled %.0f%% of all dispatches", f.TopAmbulance, f.TopAmbulanceShare*100),
			"Heavy reliance on one vehicle is a resilience risk — a breakdown would remove much of the capacity. Rebalance rosters across the fleet.")
	}
	if idle := f.FleetSize - f.ActiveAmbulances; f.FleetSize > 0 && idle > 0 {
		add("idle_fleet", "info", "Fleet",
			fmt.Sprintf("%d of %d active ambulances had no dispatch in this period", idle, f.FleetSize),
			"Check whether these vehicles are off the road, mis-registered, or available to absorb load.")
	}

	if share := pctOf(f.Incidents-f.Prioritised, f.Incidents); share >= 15 {
		add("missing_priority", "warning", "Data quality",
			fmt.Sprintf("%.0f%% of incidents have no triage priority", share),
			"Unprioritised incidents cannot be ranked on the dispatch board.")
	}
	if share := pctOf(f.Unclassified, f.Incidents); share >= 20 {
		add("unclassified", "warning", "Data quality",
			fmt.Sprintf("%.0f%% of incidents are 'Unclassified'", share),
			"Case-mix reporting (maternal, trauma, neonatal…) undercounts until intake picks a specific type.")
	}
	if withCoords := f.GPSValid + f.GPSPlaceholder; withCoords > 0 {
		if share := pctOf(f.GPSPlaceholder, f.Incidents); share >= 20 {
			add("gps_placeholder", "warning", "Data quality",
				fmt.Sprintf("%.0f%% of incidents carry placeholder GPS (0, 0)", share),
				"These cannot be mapped or used for nearest-ambulance dispatch. Fix the mobile app's location fallback.")
		}
	}

	if share := pctOf(f.MaternalNeonatal, f.Incidents); share >= 20 {
		add("mnh_burden", "info", "Clinical",
			fmt.Sprintf("Maternal & neonatal emergencies are %.0f%% of demand", share),
			"Prioritise obstetric and newborn-resuscitation skills and equipment on the busiest ambulances.")
	}

	if f.PeakHour != nil && f.PeakDow != "" {
		add("demand_peak", "info", "Demand",
			fmt.Sprintf("Demand peaks around %s and on %ss", hourLabel(*f.PeakHour), f.PeakDow),
			"Align dispatcher and crew rosters with the peak — see the Demand tab for the full hour × weekday pattern.")
	}

	if f.Assigned >= 10 && pctOf(f.AssignedWithin30, f.Assigned) >= 80 {
		add("dispatch_good", "good", "Response",
			fmt.Sprintf("%.0f%% of incidents dispatched within 30 minutes", pctOf(f.AssignedWithin30, f.Assigned)),
			"Dispatch speed is meeting the 30-minute standard.")
	}

	rank := map[string]int{"critical": 0, "warning": 1, "info": 2, "good": 3}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Severity] < rank[out[j].Severity] })
	return out
}

// ScoreQuality weights each field's completeness against its target and
// returns a 0–100 score with a letter grade.
func ScoreQuality(fields []analyticsdomain.FieldQuality) (float64, string) {
	var weighted, weights float64
	for _, fq := range fields {
		if fq.Total <= 0 || fq.Weight <= 0 {
			continue
		}
		rate := float64(fq.Filled) / float64(fq.Total)
		attainment := 1.0
		if fq.Target > 0 {
			attainment = math.Min(rate/fq.Target, 1)
		}
		weighted += attainment * fq.Weight
		weights += fq.Weight
	}
	if weights == 0 {
		return 0, "–"
	}
	score := math.Round(1000*weighted/weights) / 10
	switch {
	case score >= 90:
		return score, "A"
	case score >= 75:
		return score, "B"
	case score >= 60:
		return score, "C"
	case score >= 45:
		return score, "D"
	default:
		return score, "E"
	}
}
