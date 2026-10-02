package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	analyticsapp "dispatch/internal/modules/analytics/application"
	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// tzSQL is the zone reports bucket by on the database side.
const tzSQL = "'" + analyticsapp.ReportTimezone + "'"

// validGPS tests coordinates against Uganda's bounding box. Coordinates outside
// it (including the 0,0 default the mobile app sends when GPS is unavailable)
// are treated as placeholders.
func validGPS(alias string) string {
	return fmt.Sprintf("(%[1]s.latitude BETWEEN -1.6 AND 4.3 AND %[1]s.longitude BETWEEN 29.5 AND 35.1)", alias)
}

// Incident statuses that end a case.
const terminalStatusSQL = `('COMPLETED','CANCELLED','REJECTED')`

// Dispatch-assignment statuses that are still live.
const activeDispatchSQL = `('PROPOSED','ASSIGNED','ACCEPTED','DEPARTED','ARRIVED_SCENE','PATIENT_LOADED','ARRIVED_DESTINATION')`

// ageGroupSQL normalises the two age vocabularies in use (web form "18-59",
// mobile app "ADULT") onto five canonical bands.
const ageGroupSQL = `CASE UPPER(TRIM(COALESCE(i.patient_age_group, '')))
    WHEN '0-5' THEN 'UNDER_5' WHEN 'INFANT' THEN 'UNDER_5' WHEN 'NEONATE' THEN 'UNDER_5'
    WHEN '6-12' THEN 'CHILD' WHEN 'CHILD' THEN 'CHILD'
    WHEN '13-17' THEN 'ADOLESCENT' WHEN 'ADOLESCENT' THEN 'ADOLESCENT'
    WHEN '18-59' THEN 'ADULT' WHEN 'ADULT' THEN 'ADULT'
    WHEN '60+' THEN 'ELDERLY' WHEN 'ELDERLY' THEN 'ELDERLY'
    ELSE 'UNKNOWN' END`

// canonicalAgeVocabulary is the web-form vocabulary the data-quality report
// treats as standard.
const canonicalAgeSQL = `('0-5','6-12','13-17','18-59','60+')`

var ageGroupOrder = []struct{ Key, Label string }{
	{"UNDER_5", "Under 5"},
	{"CHILD", "6–12"},
	{"ADOLESCENT", "13–17"},
	{"ADULT", "18–59"},
	{"ELDERLY", "60+"},
	{"UNKNOWN", "Not recorded"},
}

// sqlArgs accumulates positional arguments for a dynamically built query.
type sqlArgs struct {
	values []any
}

func (a *sqlArgs) add(v any) string {
	a.values = append(a.values, v)
	return fmt.Sprintf("$%d", len(a.values))
}

// incidentWindow returns the WHERE fragment (aliased incidents AS i) for the
// given half-open window plus the district filter.
func incidentWindow(a *sqlArgs, f analyticsdomain.ReportFilters, from, to time.Time) string {
	clauses := []string{
		"i.reported_at >= " + a.add(from),
		"i.reported_at < " + a.add(to),
	}
	if f.DistrictID != nil {
		clauses = append(clauses, "i.district_id = "+a.add(*f.DistrictID))
	}
	return strings.Join(clauses, " AND ")
}

// bucketSQL renders the bucket key for a timestamp expression. Granularity is
// whitelisted by the filter resolver, so inlining it is safe.
func bucketSQL(expr, granularity string) string {
	return fmt.Sprintf("to_char(date_trunc('%s', (%s) AT TIME ZONE %s), 'YYYY-MM-DD')", granularity, expr, tzSQL)
}

// lifecycleCTE reconstructs each incident's milestones. Dispatch-assignment
// milestone columns are authoritative when present; historically they were
// rarely written, so each milestone falls back to the first matching
// STATUS_CHANGE event on the incident. The result exposes, per incident:
//
//	t_assigned … t_completed  milestone timestamps
//	m_dispatch                minutes from report to first assignment
//	m_scene                   minutes from report to arrival on scene
//	m_close                   minutes from report to completion
//
// plus the first dispatch's crew and ambulance. The caller supplies the WHERE
// fragment over incidents AS i.
func lifecycleCTE(where string) string {
	return fmt.Sprintf(`
base AS (
    SELECT i.* FROM incidents i WHERE %s
),
ev AS (
    SELECT u.incident_id,
        MIN(u.created_at) FILTER (WHERE u.new_value = 'ASSIGNED')                           AS ev_assigned,
        MIN(u.created_at) FILTER (WHERE u.new_value = 'ACCEPTED')                           AS ev_accepted,
        MIN(u.created_at) FILTER (WHERE u.new_value IN ('DEPARTED','ENROUTE'))              AS ev_departed,
        MIN(u.created_at) FILTER (WHERE u.new_value IN ('ARRIVED_SCENE','AT_SCENE'))        AS ev_scene,
        MIN(u.created_at) FILTER (WHERE u.new_value IN ('PATIENT_LOADED','TRANSPORTING'))   AS ev_loaded,
        MIN(u.created_at) FILTER (WHERE u.new_value = 'ARRIVED_DESTINATION')                AS ev_dest,
        MIN(u.created_at) FILTER (WHERE u.new_value = 'COMPLETED')                          AS ev_completed
    FROM incident_updates u
    JOIN base b ON b.id = u.incident_id
    WHERE u.update_type = 'STATUS_CHANGE'
    GROUP BY u.incident_id
),
fd AS (
    SELECT DISTINCT ON (d.incident_id) d.*
    FROM dispatch_assignments d
    JOIN base b ON b.id = d.incident_id
    ORDER BY d.incident_id, d.created_at ASC
),
ts AS (
    SELECT DISTINCT ON (s.incident_id) s.incident_id, s.triaged_at, s.total_score, s.auto_dispatch_eligible
    FROM incident_triage_sessions s
    JOIN base b ON b.id = s.incident_id
    ORDER BY s.incident_id, s.triaged_at DESC
),
lc0 AS (
    SELECT b.id, b.incident_number, b.reported_at, b.status, b.source_channel,
        b.priority_level_id, b.incident_type_id, b.district_id,
        b.referring_facility_id, b.receiving_facility_id, b.latitude, b.longitude,
        COALESCE(b.triaged_at, ts.triaged_at)                                       AS t_triaged,
        LEAST(fd.assigned_at, b.assigned_at, ev.ev_assigned)                         AS t_assigned,
        COALESCE(fd.accepted_at, ev.ev_accepted)                                     AS t_accepted,
        COALESCE(fd.departed_at, ev.ev_departed)                                     AS t_departed,
        COALESCE(fd.arrived_scene_at, ev.ev_scene)                                   AS t_scene,
        COALESCE(fd.patient_loaded_at, ev.ev_loaded)                                 AS t_loaded,
        COALESCE(fd.arrived_destination_at, ev.ev_dest)                              AS t_dest,
        COALESCE(fd.completed_at, ev.ev_completed,
                 CASE WHEN b.status = 'COMPLETED' THEN b.closed_at END)              AS t_completed,
        fd.id AS dispatch_id, fd.ambulance_id, fd.assigned_by_user_id,
        fd.driver_user_id, fd.lead_medic_user_id,
        ts.total_score, ts.auto_dispatch_eligible, (ts.incident_id IS NOT NULL) AS has_triage
    FROM base b
    LEFT JOIN ev ON ev.incident_id = b.id
    LEFT JOIN fd ON fd.incident_id = b.id
    LEFT JOIN ts ON ts.incident_id = b.id
),
lc AS (
    SELECT lc0.*,
        CASE WHEN t_assigned >= reported_at
             THEN EXTRACT(EPOCH FROM (t_assigned - reported_at)) / 60.0 END   AS m_dispatch,
        CASE WHEN t_scene >= reported_at
             THEN EXTRACT(EPOCH FROM (t_scene - reported_at)) / 60.0 END      AS m_scene,
        CASE WHEN t_completed >= reported_at
             THEN EXTRACT(EPOCH FROM (t_completed - reported_at)) / 60.0 END  AS m_close,
        COALESCE(rpl.code, 'UNASSIGNED') AS prio
    FROM lc0
    LEFT JOIN ref_priority_levels rpl ON rpl.id = lc0.priority_level_id
)`, where)
}

// queryBreakdowns runs a (key, label, count) query.
func queryBreakdowns(ctx context.Context, db *pgxpool.Pool, query string, args []any) ([]analyticsdomain.Breakdown, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []analyticsdomain.Breakdown{}
	for rows.Next() {
		var b analyticsdomain.Breakdown
		if err := rows.Scan(&b.Key, &b.Label, &b.Count); err != nil {
			return nil, err
		}
		if b.Label == "" {
			b.Label = humanizeCode(b.Key)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// queryHeat runs an (x, y, value) query.
func queryHeat(ctx context.Context, db *pgxpool.Pool, query string, args []any) ([]analyticsdomain.HeatCell, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []analyticsdomain.HeatCell{}
	for rows.Next() {
		var c analyticsdomain.HeatCell
		if err := rows.Scan(&c.X, &c.Y, &c.Value); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func isNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func ptrFloat(v float64) *float64 { return &v }

func ratio(part, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(part) / float64(total)
}

// DataStart returns the first incident's report time (within the district, if
// any) so all-time reports start where the data does.
func (r *Repository) DataStart(ctx context.Context, districtID *string) (*time.Time, error) {
	var t *time.Time
	if districtID != nil {
		err := r.db.QueryRow(ctx, `SELECT MIN(reported_at) FROM incidents WHERE district_id = $1`, *districtID).Scan(&t)
		return t, err
	}
	err := r.db.QueryRow(ctx, `SELECT MIN(reported_at) FROM incidents`).Scan(&t)
	return t, err
}
