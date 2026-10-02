package infrastructure

import (
	"context"
	"fmt"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// Demand gathers when incidents happen: hour-of-day by weekday, daily volume,
// hourly priority mix and the incident-type mix per bucket. Derived figures
// (peaks, shares, rolling average, forecast) are computed by the service.
func (r *Repository) Demand(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Demand, error) {
	out := analyticsdomain.Demand{}
	a := &sqlArgs{}
	where := incidentWindow(a, f, f.From, f.To)
	local := "(i.reported_at AT TIME ZONE " + tzSQL + ")"

	var err error
	// Hour (x, "00".."23") by ISO weekday (y, "1".."7").
	if out.HourDow, err = queryHeat(ctx, r.db, fmt.Sprintf(`
SELECT lpad(EXTRACT(HOUR FROM %[1]s)::int::text, 2, '0'), EXTRACT(ISODOW FROM %[1]s)::int::text, COUNT(*)::float8
FROM incidents i WHERE %[2]s GROUP BY 1, 2`, local, where), a.values); err != nil {
		return out, fmt.Errorf("hour/dow: %w", err)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT to_char(%[1]s, 'YYYY-MM-DD'), COUNT(*), COUNT(*) FILTER (WHERE rpl.code = 'RED')
FROM incidents i LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
WHERE %[2]s GROUP BY 1 ORDER BY 1`, local, where), a.values...)
	if err != nil {
		return out, fmt.Errorf("daily: %w", err)
	}
	for rows.Next() {
		var d analyticsdomain.DailyCount
		if err := rows.Scan(&d.Date, &d.Count, &d.Red); err != nil {
			rows.Close()
			return out, err
		}
		out.Daily = append(out.Daily, d)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	hourRows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT EXTRACT(HOUR FROM %[1]s)::int,
    COUNT(*),
    COUNT(*) FILTER (WHERE rpl.code = 'RED'),
    COUNT(*) FILTER (WHERE rpl.code = 'ORANGE'),
    COUNT(*) FILTER (WHERE rpl.code = 'GREEN'),
    COUNT(*) FILTER (WHERE rpl.code IS NULL OR rpl.code NOT IN ('RED','ORANGE','GREEN'))
FROM incidents i LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
WHERE %[2]s GROUP BY 1 ORDER BY 1`, local, where), a.values...)
	if err != nil {
		return out, fmt.Errorf("by hour: %w", err)
	}
	byHour := map[int]analyticsdomain.HourRow{}
	for hourRows.Next() {
		var h analyticsdomain.HourRow
		if err := hourRows.Scan(&h.Hour, &h.Total, &h.Red, &h.Orange, &h.Green, &h.Other); err != nil {
			hourRows.Close()
			return out, err
		}
		byHour[h.Hour] = h
	}
	hourRows.Close()
	if err := hourRows.Err(); err != nil {
		return out, err
	}
	for h := 0; h < 24; h++ {
		row, ok := byHour[h]
		if !ok {
			row = analyticsdomain.HourRow{Hour: h}
		}
		out.ByHour = append(out.ByHour, row)
	}

	if out.TypeByMonth, err = queryHeat(ctx, r.db, fmt.Sprintf(`
SELECT %[1]s, COALESCE(rit.name, 'Unknown'), COUNT(*)::float8
FROM incidents i LEFT JOIN ref_incident_types rit ON rit.id = i.incident_type_id
WHERE %[2]s GROUP BY 1, 2`, bucketSQL("i.reported_at", f.Granularity), where), a.values); err != nil {
		return out, fmt.Errorf("type by bucket: %w", err)
	}

	return out, nil
}
