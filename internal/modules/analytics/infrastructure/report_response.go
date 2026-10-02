package infrastructure

import (
	"context"
	"fmt"
	"sort"
	"strings"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// dispatchHistogram buckets report-to-assignment latency (minutes).
var dispatchHistogram = []analyticsdomain.HistogramBucket{
	{Label: "< 5 min", Min: 0, Max: 5},
	{Label: "5–15 min", Min: 5, Max: 15},
	{Label: "15–30 min", Min: 15, Max: 30},
	{Label: "30–60 min", Min: 30, Max: 60},
	{Label: "1–2 h", Min: 60, Max: 120},
	{Label: "2–4 h", Min: 120, Max: 240},
	{Label: "4–8 h", Min: 240, Max: 480},
	{Label: "8–24 h", Min: 480, Max: 1440},
	{Label: "> 24 h", Min: 1440, Max: -1},
}

var funnelStages = []struct{ Key, Label, Stamp, Prev string }{
	{"reported", "Reported", "reported_at", ""},
	{"triaged", "Triaged", "t_triaged", "reported_at"},
	{"assigned", "Ambulance assigned", "t_assigned", "reported_at"},
	{"accepted", "Crew accepted", "t_accepted", "t_assigned"},
	{"departed", "Departed / en route", "t_departed", "COALESCE(t_accepted, t_assigned)"},
	{"scene", "Arrived at scene", "t_scene", "COALESCE(t_departed, t_accepted, t_assigned)"},
	{"loaded", "Patient loaded", "t_loaded", "COALESCE(t_scene, t_departed, t_accepted, t_assigned)"},
	{"destination", "Arrived at facility", "t_dest", "COALESCE(t_loaded, t_scene, t_departed, t_accepted, t_assigned)"},
	{"completed", "Completed", "t_completed", "COALESCE(t_dest, t_loaded, t_scene, t_departed, t_accepted, t_assigned)"},
}

var priorityLabels = map[string]string{
	"RED":        "Red — high",
	"ORANGE":     "Orange — medium",
	"GREEN":      "Green — low",
	"UNASSIGNED": "Not prioritised",
}

var priorityOrder = map[string]int{"RED": 0, "ORANGE": 1, "GREEN": 2, "UNASSIGNED": 3}

// medianMinutes renders the median, in minutes, of a non-negative interval.
func medianMinutes(interval string) string {
	return fmt.Sprintf(
		"percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (%[1]s)) / 60.0) FILTER (WHERE (%[1]s) >= INTERVAL '0')",
		interval)
}

func (r *Repository) Response(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.ResponsePerformance, error) {
	out := analyticsdomain.ResponsePerformance{}
	a := &sqlArgs{}
	cte := "WITH " + lifecycleCTE(incidentWindow(a, f, f.From, f.To))

	// Milestone funnel: how many incidents reached each stage, and the median
	// time from the previous recorded stage.
	cols := []string{}
	for _, s := range funnelStages {
		cols = append(cols, fmt.Sprintf("COUNT(%s)", s.Stamp))
	}
	for _, s := range funnelStages {
		if s.Prev == "" {
			cols = append(cols, "NULL::float8")
			continue
		}
		cols = append(cols, medianMinutes(s.Stamp+" - "+s.Prev))
	}
	funnelRow := r.db.QueryRow(ctx, cte+"\nSELECT "+strings.Join(cols, ",\n    ")+"\nFROM lc", a.values...)
	counts := make([]int64, len(funnelStages))
	medians := make([]*float64, len(funnelStages))
	dest := []any{}
	for i := range counts {
		dest = append(dest, &counts[i])
	}
	for i := range medians {
		dest = append(dest, &medians[i])
	}
	if err := funnelRow.Scan(dest...); err != nil {
		return out, fmt.Errorf("funnel: %w", err)
	}
	for i, s := range funnelStages {
		out.Funnel = append(out.Funnel, analyticsdomain.FunnelStage{
			Key: s.Key, Label: s.Label, Count: counts[i], MedianMinutes: medians[i],
		})
	}

	// Headline percentiles for dispatch, scene arrival and closure.
	var err error
	if out.Dispatch, err = r.percentiles(ctx, cte, "m_dispatch", a.values); err != nil {
		return out, fmt.Errorf("dispatch percentiles: %w", err)
	}
	if out.SceneArrival, err = r.percentiles(ctx, cte, "m_scene", a.values); err != nil {
		return out, fmt.Errorf("scene percentiles: %w", err)
	}
	if out.Closure, err = r.percentiles(ctx, cte, "m_close", a.values); err != nil {
		return out, fmt.Errorf("closure percentiles: %w", err)
	}

	// Dispatch latency histogram.
	hcols := []string{}
	for _, b := range dispatchHistogram {
		if b.Max < 0 {
			hcols = append(hcols, fmt.Sprintf("COUNT(*) FILTER (WHERE m_dispatch >= %d)", b.Min))
		} else {
			hcols = append(hcols, fmt.Sprintf("COUNT(*) FILTER (WHERE m_dispatch >= %d AND m_dispatch < %d)", b.Min, b.Max))
		}
	}
	hcounts := make([]int64, len(dispatchHistogram))
	hdest := []any{}
	for i := range hcounts {
		hdest = append(hdest, &hcounts[i])
	}
	if err := r.db.QueryRow(ctx, cte+"\nSELECT "+strings.Join(hcols, ", ")+" FROM lc", a.values...).Scan(hdest...); err != nil {
		return out, fmt.Errorf("histogram: %w", err)
	}
	for i, b := range dispatchHistogram {
		b.Count = hcounts[i]
		out.Histogram = append(out.Histogram, b)
	}

	// Service-level compliance by priority and by intake channel.
	if out.SLAByPriority, err = r.slaRows(ctx, cte, "prio", a.values); err != nil {
		return out, fmt.Errorf("sla priority: %w", err)
	}
	for i := range out.SLAByPriority {
		if label, ok := priorityLabels[out.SLAByPriority[i].Key]; ok {
			out.SLAByPriority[i].Label = label
		}
	}
	sortByPriority(out.SLAByPriority)
	if out.SLAByChannel, err = r.slaRows(ctx, cte, "source_channel", a.values); err != nil {
		return out, fmt.Errorf("sla channel: %w", err)
	}

	// Per-bucket distribution of dispatch latency.
	boxRows, err := r.db.Query(ctx, cte+fmt.Sprintf(`
SELECT %s, COUNT(m_dispatch),
    percentile_cont(ARRAY[0.1, 0.25, 0.5, 0.75, 0.9]) WITHIN GROUP (ORDER BY m_dispatch)
FROM lc WHERE m_dispatch IS NOT NULL GROUP BY 1 ORDER BY 1`, bucketSQL("reported_at", f.Granularity)), a.values...)
	if err != nil {
		return out, fmt.Errorf("box: %w", err)
	}
	for boxRows.Next() {
		var b analyticsdomain.BoxRow
		var q []float64
		if err := boxRows.Scan(&b.Bucket, &b.N, &q); err != nil {
			boxRows.Close()
			return out, err
		}
		if len(q) == 5 {
			b.P10, b.Q1, b.Median, b.Q3, b.P90 = q[0], q[1], q[2], q[3], q[4]
		}
		out.Box = append(out.Box, b)
	}
	boxRows.Close()
	if err := boxRows.Err(); err != nil {
		return out, err
	}

	// Dispatcher workload and speed.
	dispRows, err := r.db.Query(ctx, cte+`
SELECT lc.assigned_by_user_id::text,
    COALESCE(NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.username, 'Unknown'),
    COUNT(*),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY m_dispatch),
    COUNT(*) FILTER (WHERE m_dispatch <= 30),
    MAX(t_assigned)
FROM lc LEFT JOIN users u ON u.id = lc.assigned_by_user_id
WHERE lc.assigned_by_user_id IS NOT NULL
GROUP BY 1, 2 ORDER BY 3 DESC`, a.values...)
	if err != nil {
		return out, fmt.Errorf("dispatchers: %w", err)
	}
	for dispRows.Next() {
		p := analyticsdomain.PersonRow{Role: "DISPATCHER"}
		if err := dispRows.Scan(&p.UserID, &p.Name, &p.Count, &p.MedianMinutes, &p.Within30, &p.LastAt); err != nil {
			dispRows.Close()
			return out, err
		}
		out.Dispatchers = append(out.Dispatchers, p)
	}
	dispRows.Close()
	if err := dispRows.Err(); err != nil {
		return out, err
	}

	// The slowest dispatches, for case review.
	slowRows, err := r.db.Query(ctx, cte+`
SELECT lc.id::text, lc.incident_number, lc.reported_at, lc.m_dispatch,
    lc.prio, COALESCE(rit.name, 'Unknown'), lc.status
FROM lc LEFT JOIN ref_incident_types rit ON rit.id = lc.incident_type_id
WHERE lc.m_dispatch IS NOT NULL
ORDER BY lc.m_dispatch DESC LIMIT 10`, a.values...)
	if err != nil {
		return out, fmt.Errorf("slowest: %w", err)
	}
	for slowRows.Next() {
		var s analyticsdomain.SlowCase
		if err := slowRows.Scan(&s.IncidentID, &s.IncidentNumber, &s.ReportedAt, &s.Minutes, &s.Priority, &s.Type, &s.Status); err != nil {
			slowRows.Close()
			return out, err
		}
		out.Slowest = append(out.Slowest, s)
	}
	slowRows.Close()
	return out, slowRows.Err()
}

func (r *Repository) percentiles(ctx context.Context, cte, column string, args []any) (analyticsdomain.Percentiles, error) {
	p := analyticsdomain.Percentiles{}
	var q []float64
	err := r.db.QueryRow(ctx, cte+fmt.Sprintf(`
SELECT COUNT(%[1]s), AVG(%[1]s),
    percentile_cont(ARRAY[0.5, 0.75, 0.9]) WITHIN GROUP (ORDER BY %[1]s)
FROM lc WHERE %[1]s IS NOT NULL`, column), args...).Scan(&p.N, &p.Mean, &q)
	if err != nil {
		return p, err
	}
	if len(q) == 3 {
		p.P50, p.P75, p.P90 = ptrFloat(q[0]), ptrFloat(q[1]), ptrFloat(q[2])
	}
	return p, nil
}

func (r *Repository) slaRows(ctx context.Context, cte, groupBy string, args []any) ([]analyticsdomain.SLARow, error) {
	rows, err := r.db.Query(ctx, cte+fmt.Sprintf(`
SELECT COALESCE(%[1]s, 'UNKNOWN'),
    COUNT(m_dispatch),
    COUNT(*) FILTER (WHERE m_dispatch <= 15),
    COUNT(*) FILTER (WHERE m_dispatch <= 30),
    COUNT(*) FILTER (WHERE m_dispatch <= 60),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY m_dispatch),
    percentile_cont(0.9) WITHIN GROUP (ORDER BY m_dispatch)
FROM lc GROUP BY 1 ORDER BY 2 DESC`, groupBy), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []analyticsdomain.SLARow{}
	for rows.Next() {
		var s analyticsdomain.SLARow
		if err := rows.Scan(&s.Key, &s.N, &s.Within15, &s.Within30, &s.Within60, &s.Median, &s.P90); err != nil {
			return nil, err
		}
		s.Label = humanizeCode(s.Key)
		out = append(out, s)
	}
	return out, rows.Err()
}

func sortByPriority(rows []analyticsdomain.SLARow) {
	rank := func(k string) int {
		if v, ok := priorityOrder[k]; ok {
			return v
		}
		return len(priorityOrder)
	}
	sort.SliceStable(rows, func(i, j int) bool { return rank(rows[i].Key) < rank(rows[j].Key) })
}
