package infrastructure

import (
	"context"
	"fmt"
	"time"

	analyticsapp "dispatch/internal/modules/analytics/application"
	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// Overview gathers the executive overview: current vs previous-period facts,
// the bucketed trend, and the headline distributions. KPIs and insights are
// derived from these facts in the service layer.
func (r *Repository) Overview(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Overview, error) {
	out := analyticsdomain.Overview{}

	if err := r.overviewFacts(ctx, f, &out.Facts); err != nil {
		return out, fmt.Errorf("overview facts: %w", err)
	}
	if err := r.overviewFleetFacts(ctx, f, &out.Facts); err != nil {
		return out, fmt.Errorf("overview fleet: %w", err)
	}
	trend, err := r.overviewTrend(ctx, f)
	if err != nil {
		return out, fmt.Errorf("overview trend: %w", err)
	}
	out.Trend = trend

	a := &sqlArgs{}
	where := incidentWindow(a, f, f.From, f.To)

	if out.ByPriority, err = queryBreakdowns(ctx, r.db, fmt.Sprintf(`
SELECT COALESCE(rpl.code, 'UNASSIGNED'), COALESCE(rpl.name, 'Not prioritised'), COUNT(*)
FROM incidents i LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
WHERE %s GROUP BY rpl.code, rpl.name, rpl.sort_order ORDER BY rpl.sort_order NULLS LAST`, where), a.values); err != nil {
		return out, err
	}
	if out.ByType, err = queryBreakdowns(ctx, r.db, fmt.Sprintf(`
SELECT COALESCE(rit.code, 'UNKNOWN'), COALESCE(rit.name, 'Unknown'), COUNT(*)
FROM incidents i LEFT JOIN ref_incident_types rit ON rit.id = i.incident_type_id
WHERE %s GROUP BY rit.code, rit.name ORDER BY COUNT(*) DESC`, where), a.values); err != nil {
		return out, err
	}
	if out.ByStatus, err = queryBreakdowns(ctx, r.db, fmt.Sprintf(`
SELECT i.status, '', COUNT(*) FROM incidents i WHERE %s GROUP BY i.status ORDER BY COUNT(*) DESC`, where), a.values); err != nil {
		return out, err
	}
	if out.ByChannel, err = queryBreakdowns(ctx, r.db, fmt.Sprintf(`
SELECT i.source_channel, '', COUNT(*) FROM incidents i WHERE %s GROUP BY i.source_channel ORDER BY COUNT(*) DESC`, where), a.values); err != nil {
		return out, err
	}
	if out.ByDistrict, err = queryBreakdowns(ctx, r.db, fmt.Sprintf(`
SELECT COALESCE(rd.id::text, 'UNASSIGNED'), COALESCE(rd.name, 'Not recorded'), COUNT(*)
FROM incidents i LEFT JOIN ref_districts rd ON rd.id = i.district_id
WHERE %s GROUP BY rd.id, rd.name ORDER BY COUNT(*) DESC LIMIT 15`, where), a.values); err != nil {
		return out, err
	}

	return out, nil
}

func (r *Repository) overviewFacts(ctx context.Context, f analyticsdomain.ReportFilters, facts *analyticsdomain.OverviewFacts) error {
	a := &sqlArgs{}
	where := incidentWindow(a, f, f.PrevFrom, f.To)
	cur := a.add(f.From)
	staleBefore := a.add(time.Now().Add(-24 * time.Hour))

	query := "WITH " + lifecycleCTE(where) + fmt.Sprintf(`,
fb AS (SELECT DISTINCT incident_id FROM incident_feedback),
x AS (
    SELECT lc.*, (lc.reported_at >= %[1]s) AS cur, rit.code AS type_code,
        (fb.incident_id IS NOT NULL) AS has_feedback
    FROM lc
    LEFT JOIN ref_incident_types rit ON rit.id = lc.incident_type_id
    LEFT JOIN fb ON fb.incident_id = lc.id
)
SELECT
    COUNT(*) FILTER (WHERE cur),
    COUNT(*) FILTER (WHERE NOT cur),
    COUNT(*) FILTER (WHERE cur AND prio = 'RED'),
    COUNT(*) FILTER (WHERE NOT cur AND prio = 'RED'),
    COUNT(*) FILTER (WHERE cur AND prio <> 'UNASSIGNED'),
    COUNT(*) FILTER (WHERE NOT cur AND prio <> 'UNASSIGNED'),
    COUNT(*) FILTER (WHERE cur AND status = 'COMPLETED'),
    COUNT(*) FILTER (WHERE NOT cur AND status = 'COMPLETED'),
    COUNT(*) FILTER (WHERE cur AND status NOT IN ('CANCELLED','REJECTED')),
    COUNT(*) FILTER (WHERE NOT cur AND status NOT IN ('CANCELLED','REJECTED')),
    COUNT(*) FILTER (WHERE cur AND referring_facility_id IS NOT NULL AND receiving_facility_id IS NOT NULL),
    COUNT(*) FILTER (WHERE NOT cur AND referring_facility_id IS NOT NULL AND receiving_facility_id IS NOT NULL),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY m_dispatch) FILTER (WHERE cur),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY m_dispatch) FILTER (WHERE NOT cur),
    COUNT(m_dispatch) FILTER (WHERE cur),
    COUNT(m_dispatch) FILTER (WHERE NOT cur),
    COUNT(*) FILTER (WHERE cur AND m_dispatch <= 30),
    COUNT(*) FILTER (WHERE NOT cur AND m_dispatch <= 30),
    COUNT(*) FILTER (WHERE cur AND status = 'COMPLETED' AND has_feedback),
    COUNT(*) FILTER (WHERE NOT cur AND status = 'COMPLETED' AND has_feedback),
    COUNT(*) FILTER (WHERE cur AND type_code = 'UNCLASSIFIED'),
    COUNT(*) FILTER (WHERE cur AND type_code IN ('MATERNAL_EMERGENCY','NEONATAL_EMERGENCY')),
    COUNT(*) FILTER (WHERE cur AND %[3]s AND NOT (latitude = 0 AND longitude = 0)),
    COUNT(*) FILTER (WHERE cur AND latitude IS NOT NULL AND NOT %[3]s),
    COUNT(*) FILTER (WHERE cur AND status NOT IN %[4]s AND reported_at < %[2]s)
FROM x`, cur, staleBefore, validGPS("x"), terminalStatusSQL)

	return r.db.QueryRow(ctx, query, a.values...).Scan(
		&facts.Incidents, &facts.PrevIncidents,
		&facts.Red, &facts.PrevRed,
		&facts.Prioritised, &facts.PrevPrioritised,
		&facts.Completed, &facts.PrevCompleted,
		&facts.Closable, &facts.PrevClosable,
		&facts.Referrals, &facts.PrevReferrals,
		&facts.MedianDispatch, &facts.PrevMedianDispatch,
		&facts.Assigned, &facts.PrevAssigned,
		&facts.AssignedWithin30, &facts.PrevAssignedWithin,
		&facts.WithFeedback, &facts.PrevWithFeedback,
		&facts.Unclassified,
		&facts.MaternalNeonatal,
		&facts.GPSValid,
		&facts.GPSPlaceholder,
		&facts.OpenStale,
	)
}

func (r *Repository) overviewFleetFacts(ctx context.Context, f analyticsdomain.ReportFilters, facts *analyticsdomain.OverviewFacts) error {
	// Dispatches in both windows (by the incident's report time).
	a := &sqlArgs{}
	where := incidentWindow(a, f, f.PrevFrom, f.To)
	cur := a.add(f.From)
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
SELECT COUNT(*) FILTER (WHERE i.reported_at >= %s), COUNT(*) FILTER (WHERE i.reported_at < %s)
FROM dispatch_assignments d JOIN incidents i ON i.id = d.incident_id
WHERE %s`, cur, cur, where), a.values...).Scan(&facts.Dispatches, &facts.PrevDispatches); err != nil {
		return err
	}

	// Fuel spend in both windows, scoped to the district's ambulances.
	fa := &sqlArgs{}
	fuelWhere := "TRUE"
	if f.DistrictID != nil {
		fuelWhere = "a.district_id = " + fa.add(*f.DistrictID)
	}
	q := fmt.Sprintf(`
SELECT
    COALESCE(SUM(fl.cost) FILTER (WHERE fl.filled_at >= %s AND fl.filled_at < %s), 0),
    COALESCE(SUM(fl.cost) FILTER (WHERE fl.filled_at >= %s AND fl.filled_at < %s), 0)
FROM fuel_logs fl LEFT JOIN ambulances a ON a.id = fl.ambulance_id
WHERE %s`, fa.add(f.From), fa.add(f.To), fa.add(f.PrevFrom), fa.add(f.PrevTo), fuelWhere)
	if err := r.db.QueryRow(ctx, q, fa.values...).Scan(&facts.FuelCost, &facts.PrevFuelCost); err != nil {
		return err
	}

	// Fleet size.
	ca := &sqlArgs{}
	fleetWhere := "a.is_active"
	if f.DistrictID != nil {
		fleetWhere += " AND a.district_id = " + ca.add(*f.DistrictID)
	}
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM ambulances a WHERE `+fleetWhere, ca.values...).Scan(&facts.FleetSize); err != nil {
		return err
	}

	// Active ambulances and the busiest one in the current window.
	ba := &sqlArgs{}
	bw := incidentWindow(ba, f, f.From, f.To)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT COALESCE(a.plate_number, d.ambulance_id::text), COUNT(*)
FROM dispatch_assignments d
JOIN incidents i ON i.id = d.incident_id
LEFT JOIN ambulances a ON a.id = d.ambulance_id
WHERE %s AND d.ambulance_id IS NOT NULL
GROUP BY 1 ORDER BY 2 DESC`, bw), ba.values...)
	if err != nil {
		return err
	}
	defer rows.Close()
	var total, top int64
	for rows.Next() {
		var plate string
		var n int64
		if err := rows.Scan(&plate, &n); err != nil {
			return err
		}
		if facts.TopAmbulance == "" {
			facts.TopAmbulance, top = plate, n
		}
		total += n
		facts.ActiveAmbulances++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	facts.TopAmbulanceShare = ratio(top, total)

	// Demand peaks for the roster insight.
	pa := &sqlArgs{}
	pw := incidentWindow(pa, f, f.From, f.To)
	var peakHour *int
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
SELECT EXTRACT(HOUR FROM i.reported_at AT TIME ZONE %s)::int
FROM incidents i WHERE %s GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 1`, tzSQL, pw), pa.values...).Scan(&peakHour); err != nil && !isNoRows(err) {
		return err
	}
	facts.PeakHour = peakHour
	var peakDow string
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
SELECT trim(to_char(i.reported_at AT TIME ZONE %s, 'Day'))
FROM incidents i WHERE %s GROUP BY 1 ORDER BY COUNT(*) DESC, 1 LIMIT 1`, tzSQL, pw), pa.values...).Scan(&peakDow); err != nil && !isNoRows(err) {
		return err
	}
	facts.PeakDow = peakDow
	return nil
}

func (r *Repository) overviewTrend(ctx context.Context, f analyticsdomain.ReportFilters) ([]analyticsdomain.TrendPoint, error) {
	a := &sqlArgs{}
	where := incidentWindow(a, f, f.From, f.To)
	query := "WITH " + lifecycleCTE(where) + fmt.Sprintf(`
SELECT %s AS bucket,
    COUNT(*),
    COUNT(*) FILTER (WHERE prio = 'RED'),
    COUNT(*) FILTER (WHERE prio = 'ORANGE'),
    COUNT(*) FILTER (WHERE prio = 'GREEN'),
    COUNT(*) FILTER (WHERE prio NOT IN ('RED','ORANGE','GREEN')),
    COUNT(*) FILTER (WHERE status = 'COMPLETED'),
    COUNT(*) FILTER (WHERE referring_facility_id IS NOT NULL AND receiving_facility_id IS NOT NULL),
    COUNT(m_dispatch),
    COUNT(*) FILTER (WHERE m_dispatch <= 30),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY m_dispatch)
FROM lc GROUP BY 1`, bucketSQL("reported_at", f.Granularity))

	rows, err := r.db.Query(ctx, query, a.values...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	byBucket := map[string]analyticsdomain.TrendPoint{}
	for rows.Next() {
		var p analyticsdomain.TrendPoint
		if err := rows.Scan(&p.Bucket, &p.Total, &p.Red, &p.Orange, &p.Green, &p.Unprioritised,
			&p.Completed, &p.Referrals, &p.Assigned, &p.AssignedWithin30, &p.MedianDispatch); err != nil {
			return nil, err
		}
		byBucket[p.Bucket] = p
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fuel spend per bucket.
	fa := &sqlArgs{}
	fuelWhere := fmt.Sprintf("fl.filled_at >= %s AND fl.filled_at < %s", fa.add(f.From), fa.add(f.To))
	if f.DistrictID != nil {
		fuelWhere += " AND a.district_id = " + fa.add(*f.DistrictID)
	}
	fuelRows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT %s, COALESCE(SUM(fl.cost), 0)
FROM fuel_logs fl LEFT JOIN ambulances a ON a.id = fl.ambulance_id
WHERE %s GROUP BY 1`, bucketSQL("fl.filled_at", f.Granularity), fuelWhere), fa.values...)
	if err != nil {
		return nil, err
	}
	defer fuelRows.Close()
	fuel := map[string]float64{}
	for fuelRows.Next() {
		var bucket string
		var cost float64
		if err := fuelRows.Scan(&bucket, &cost); err != nil {
			return nil, err
		}
		fuel[bucket] = cost
	}
	if err := fuelRows.Err(); err != nil {
		return nil, err
	}

	out := []analyticsdomain.TrendPoint{}
	for _, bucket := range analyticsapp.Buckets(f) {
		p, ok := byBucket[bucket]
		if !ok {
			p = analyticsdomain.TrendPoint{Bucket: bucket}
		}
		p.FuelCost = fuel[bucket]
		out = append(out, p)
	}
	return out, nil
}
