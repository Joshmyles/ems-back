package infrastructure

import (
	"context"
	"fmt"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// referralJoins resolves origin/destination facilities, their levels and the
// incident priority for incidents AS i.
const referralJoins = `
LEFT JOIN ref_facilities o ON o.id = i.referring_facility_id
LEFT JOIN ref_facility_levels ol ON ol.id = o.level_id
LEFT JOIN ref_facilities d ON d.id = i.receiving_facility_id
LEFT JOIN ref_facility_levels dl ON dl.id = d.level_id
LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id`

// Referrals maps the inter-facility transfer network: who refers to whom, how
// patients move between levels of care, and the busiest facilities.
func (r *Repository) Referrals(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.ReferralNetwork, error) {
	out := analyticsdomain.ReferralNetwork{}
	a := &sqlArgs{}
	where := incidentWindow(a, f, f.From, f.To)

	t := &out.Totals
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
SELECT
    COUNT(*),
    COUNT(*) FILTER (WHERE i.referring_facility_id IS NOT NULL AND i.receiving_facility_id IS NOT NULL),
    COUNT(i.referring_facility_id),
    COUNT(i.receiving_facility_id),
    COUNT(DISTINCT i.referring_facility_id),
    COUNT(DISTINCT i.receiving_facility_id),
    COUNT(*) FILTER (WHERE ol.rank_no < dl.rank_no),
    COUNT(*) FILTER (WHERE ol.rank_no = dl.rank_no),
    COUNT(*) FILTER (WHERE ol.rank_no > dl.rank_no),
    COUNT(*) FILTER (WHERE i.referring_facility_id IS NOT NULL AND i.receiving_facility_id IS NOT NULL AND rpl.code = 'RED'),
    COUNT(*) FILTER (WHERE dl.code = 'NRH')
FROM incidents i %s
WHERE %s`, referralJoins, where), a.values...).Scan(
		&t.Incidents, &t.Transfers, &t.WithOrigin, &t.WithDestination, &t.Origins, &t.Destinations,
		&t.Upward, &t.Lateral, &t.Downward, &t.Critical, &t.ToNational,
	); err != nil {
		return out, fmt.Errorf("referral totals: %w", err)
	}

	var err error
	if out.Flows, err = r.referralFlows(ctx, where, a.values, 30); err != nil {
		return out, fmt.Errorf("flows: %w", err)
	}

	levelRows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT COALESCE(ol.code, 'UNKNOWN'), COALESCE(ol.name, 'Unknown level'),
    COALESCE(dl.code, 'UNKNOWN'), COALESCE(dl.name, 'Unknown level'),
    COUNT(*), COUNT(*) FILTER (WHERE rpl.code = 'RED')
FROM incidents i %s
WHERE %s AND i.referring_facility_id IS NOT NULL AND i.receiving_facility_id IS NOT NULL
GROUP BY 1, 2, 3, 4, ol.rank_no, dl.rank_no
ORDER BY ol.rank_no NULLS LAST, dl.rank_no NULLS LAST`, referralJoins, where), a.values...)
	if err != nil {
		return out, fmt.Errorf("level flows: %w", err)
	}
	for levelRows.Next() {
		var fl analyticsdomain.Flow
		if err := levelRows.Scan(&fl.SourceID, &fl.Source, &fl.TargetID, &fl.Target, &fl.Count, &fl.Critical); err != nil {
			levelRows.Close()
			return out, err
		}
		fl.SourceLevel, fl.TargetLevel = fl.Source, fl.Target
		out.LevelFlows = append(out.LevelFlows, fl)
		out.LevelMatrix = append(out.LevelMatrix, analyticsdomain.HeatCell{X: fl.Target, Y: fl.Source, Value: float64(fl.Count)})
	}
	levelRows.Close()
	if err := levelRows.Err(); err != nil {
		return out, err
	}

	if out.Facilities, err = r.referralFacilities(ctx, where, a.values, 60); err != nil {
		return out, fmt.Errorf("facilities: %w", err)
	}

	if out.Reasons, err = queryBreakdowns(ctx, r.db, fmt.Sprintf(`
SELECT COALESCE(rit.code, 'UNKNOWN'), COALESCE(rit.name, 'Unknown'), COUNT(*)
FROM incidents i LEFT JOIN ref_incident_types rit ON rit.id = i.incident_type_id
WHERE %s AND i.referring_facility_id IS NOT NULL AND i.receiving_facility_id IS NOT NULL
GROUP BY 1, 2 ORDER BY 3 DESC`, where), a.values); err != nil {
		return out, fmt.Errorf("reasons: %w", err)
	}

	return out, nil
}

func (r *Repository) referralFlows(ctx context.Context, where string, args []any, limit int) ([]analyticsdomain.Flow, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT o.id::text, o.name, COALESCE(ol.name, ''), d.id::text, d.name, COALESCE(dl.name, ''),
    COUNT(*), COUNT(*) FILTER (WHERE rpl.code = 'RED')
FROM incidents i %s
WHERE %s AND o.id IS NOT NULL AND d.id IS NOT NULL
GROUP BY o.id, o.name, ol.name, d.id, d.name, dl.name
ORDER BY COUNT(*) DESC, o.name, d.name
LIMIT %d`, referralJoins, where, limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []analyticsdomain.Flow{}
	for rows.Next() {
		var fl analyticsdomain.Flow
		if err := rows.Scan(&fl.SourceID, &fl.Source, &fl.SourceLevel, &fl.TargetID, &fl.Target, &fl.TargetLevel, &fl.Count, &fl.Critical); err != nil {
			return nil, err
		}
		out = append(out, fl)
	}
	return out, rows.Err()
}

func (r *Repository) referralFacilities(ctx context.Context, where string, args []any, limit int) ([]analyticsdomain.FacilityRow, error) {
	rows, err := r.db.Query(ctx, fmt.Sprintf(`
WITH touches AS (
    SELECT i.referring_facility_id AS fid, 1 AS sent, 0 AS received, COALESCE((rpl.code = 'RED')::int, 0) AS critical
    FROM incidents i LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
    WHERE %[1]s AND i.referring_facility_id IS NOT NULL
    UNION ALL
    SELECT i.receiving_facility_id, 0, 1, COALESCE((rpl.code = 'RED')::int, 0)
    FROM incidents i LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
    WHERE %[1]s AND i.receiving_facility_id IS NOT NULL
)
SELECT rf.id::text, rf.name, COALESCE(lv.name, ''), COALESCE(rd.name, ''),
    SUM(t.sent), SUM(t.received), SUM(t.critical),
    CASE WHEN %[2]s THEN rf.latitude END,
    CASE WHEN %[2]s THEN rf.longitude END
FROM touches t
JOIN ref_facilities rf ON rf.id = t.fid
LEFT JOIN ref_facility_levels lv ON lv.id = rf.level_id
LEFT JOIN ref_districts rd ON rd.id = rf.district_id
GROUP BY rf.id, rf.name, lv.name, rd.name, rf.latitude, rf.longitude
ORDER BY SUM(t.sent) + SUM(t.received) DESC, rf.name
LIMIT %[3]d`, where, validGPS("rf"), limit), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []analyticsdomain.FacilityRow{}
	for rows.Next() {
		var fr analyticsdomain.FacilityRow
		if err := rows.Scan(&fr.ID, &fr.Name, &fr.Level, &fr.District, &fr.Sent, &fr.Received, &fr.Critical, &fr.Latitude, &fr.Longitude); err != nil {
			return nil, err
		}
		out = append(out, fr)
	}
	return out, rows.Err()
}

// Geo returns map layers: incident locations with valid coordinates, the
// referral facilities, and the flows between them.
func (r *Repository) Geo(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Geo, error) {
	out := analyticsdomain.Geo{}
	a := &sqlArgs{}
	where := incidentWindow(a, f, f.From, f.To)

	if err := r.db.QueryRow(ctx, fmt.Sprintf(`
SELECT COUNT(*),
    COUNT(*) FILTER (WHERE i.latitude IS NULL OR i.longitude IS NULL),
    COUNT(*) FILTER (WHERE i.latitude IS NOT NULL AND i.longitude IS NOT NULL
                       AND (NOT %[1]s OR (i.latitude = 0 AND i.longitude = 0)))
FROM incidents i WHERE %[2]s`, validGPS("i"), where), a.values...).Scan(&out.Total, &out.Missing, &out.Placeholder); err != nil {
		return out, fmt.Errorf("geo counts: %w", err)
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT i.id::text, i.incident_number, i.latitude, i.longitude,
    COALESCE(rpl.code, 'UNASSIGNED'), COALESCE(rit.name, 'Unknown'), i.status, i.reported_at
FROM incidents i
LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
LEFT JOIN ref_incident_types rit ON rit.id = i.incident_type_id
WHERE %s AND %s AND NOT (i.latitude = 0 AND i.longitude = 0)
ORDER BY i.reported_at DESC
LIMIT 5000`, where, validGPS("i")), a.values...)
	if err != nil {
		return out, fmt.Errorf("geo points: %w", err)
	}
	for rows.Next() {
		var p analyticsdomain.GeoPoint
		if err := rows.Scan(&p.ID, &p.Number, &p.Latitude, &p.Longitude, &p.Priority, &p.Type, &p.Status, &p.ReportedAt); err != nil {
			rows.Close()
			return out, err
		}
		out.Points = append(out.Points, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	if out.Facilities, err = r.referralFacilities(ctx, where, a.values, 200); err != nil {
		return out, fmt.Errorf("geo facilities: %w", err)
	}
	if out.Flows, err = r.referralFlows(ctx, where, a.values, 200); err != nil {
		return out, fmt.Errorf("geo flows: %w", err)
	}
	return out, nil
}
