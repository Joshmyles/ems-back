package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	analyticsapp "dispatch/internal/modules/analytics/application"
	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// Statuses considered "in progress" (i.e. the patient case is still live).
const openStatusFilter = "i.status NOT IN ('COMPLETED','CANCELLED','REJECTED')"

// Statuses from assignment onwards — a responder has been engaged.
const assignedStatusFilter = "i.status IN ('ASSIGNED','ENROUTE','AT_SCENE','TRANSPORTING','COMPLETED')"

// Statuses where the patient is/was physically moved.
const transportedStatusFilter = "i.status IN ('TRANSPORTING','COMPLETED')"

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

var _ analyticsapp.Repository = (*Repository)(nil)

// buildFilter assembles the shared WHERE clause (aliased to incidents AS i) and
// its positional args, reused across every aggregate query.
func buildFilter(f analyticsdomain.Filters) (string, []any) {
	clauses := []string{"1=1"}
	args := []any{}
	pos := 1

	if f.DateFrom != nil {
		clauses = append(clauses, fmt.Sprintf("i.reported_at >= $%d", pos))
		args = append(args, *f.DateFrom)
		pos++
	}
	if f.DateTo != nil {
		clauses = append(clauses, fmt.Sprintf("i.reported_at <= $%d", pos))
		args = append(args, *f.DateTo)
		pos++
	}
	if f.DistrictID != nil {
		clauses = append(clauses, fmt.Sprintf("i.district_id = $%d", pos))
		args = append(args, *f.DistrictID)
		pos++
	}

	return strings.Join(clauses, " AND "), args
}

func (r *Repository) GetSummary(ctx context.Context, filters analyticsdomain.Filters) (analyticsdomain.Summary, error) {
	out := analyticsdomain.Summary{}
	where, args := buildFilter(filters)

	if err := r.loadTotals(ctx, &out, where, args); err != nil {
		return out, err
	}
	if err := r.loadStatus(ctx, &out, where, args); err != nil {
		return out, err
	}
	if err := r.loadPriority(ctx, &out, where, args); err != nil {
		return out, err
	}
	if err := r.loadSeverity(ctx, &out, where, args); err != nil {
		return out, err
	}
	if err := r.loadType(ctx, &out, where, args); err != nil {
		return out, err
	}
	if err := r.loadDistricts(ctx, &out, where, args); err != nil {
		return out, err
	}
	if err := r.loadPatient(ctx, &out, where, args); err != nil {
		return out, err
	}

	return out, nil
}

func (r *Repository) loadTotals(ctx context.Context, out *analyticsdomain.Summary, where string, args []any) error {
	query := fmt.Sprintf(`
SELECT
    COUNT(*),
    COUNT(*) FILTER (WHERE %s),
    COUNT(*) FILTER (WHERE %s),
    COUNT(*) FILTER (WHERE i.status = 'TRANSPORTING'),
    COUNT(*) FILTER (WHERE i.status = 'COMPLETED'),
    COUNT(*) FILTER (WHERE i.status = 'CANCELLED'),
    COUNT(*) FILTER (WHERE i.referring_facility_id IS NOT NULL),
    COUNT(*) FILTER (WHERE i.receiving_facility_id IS NOT NULL),
    COUNT(*) FILTER (WHERE rpl.code = 'RED'),
    COUNT(*) FILTER (WHERE i.verification_status = 'VERIFIED'),
    COUNT(*) FILTER (WHERE i.verification_status = 'PENDING'),
    AVG(EXTRACT(EPOCH FROM (i.assigned_at - i.reported_at)) / 60.0)
        FILTER (WHERE i.assigned_at IS NOT NULL AND i.assigned_at >= i.reported_at),
    AVG(EXTRACT(EPOCH FROM (i.closed_at - i.reported_at)) / 60.0)
        FILTER (WHERE i.closed_at IS NOT NULL AND i.closed_at >= i.reported_at)
FROM incidents i
LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
WHERE %s`, openStatusFilter, assignedStatusFilter, where)

	t := &out.Totals
	return r.db.QueryRow(ctx, query, args...).Scan(
		&t.Incidents,
		&t.Open,
		&t.Assigned,
		&t.InTransport,
		&t.Completed,
		&t.Cancelled,
		&t.Referrals,
		&t.Transfers,
		&t.Critical,
		&t.Verified,
		&t.PendingVerification,
		&out.ResponseTimes.AvgMinutesToAssignment,
		&out.ResponseTimes.AvgMinutesToClosure,
	)
}

func (r *Repository) loadStatus(ctx context.Context, out *analyticsdomain.Summary, where string, args []any) error {
	query := fmt.Sprintf(`
SELECT i.status, COUNT(*)
FROM incidents i
WHERE %s
GROUP BY i.status
ORDER BY COUNT(*) DESC`, where)

	rows, err := keyCount(ctx, r.db, query, args, true)
	if err != nil {
		return err
	}
	out.ByStatus = rows
	return nil
}

func (r *Repository) loadPriority(ctx context.Context, out *analyticsdomain.Summary, where string, args []any) error {
	query := fmt.Sprintf(`
SELECT COALESCE(rpl.code, 'UNASSIGNED'), COALESCE(rpl.name, 'Unassigned'), COUNT(*)
FROM incidents i
LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
WHERE %s
GROUP BY rpl.code, rpl.name, rpl.sort_order
ORDER BY rpl.sort_order NULLS LAST`, where)

	rows, err := keyLabelCount(ctx, r.db, query, args)
	if err != nil {
		return err
	}
	out.ByPriority = rows
	return nil
}

func (r *Repository) loadSeverity(ctx context.Context, out *analyticsdomain.Summary, where string, args []any) error {
	query := fmt.Sprintf(`
SELECT COALESCE(rsl.code, 'UNASSIGNED'), COALESCE(rsl.name, 'Unassigned'), COUNT(*)
FROM incidents i
LEFT JOIN ref_severity_levels rsl ON rsl.id = i.severity_level_id
WHERE %s
GROUP BY rsl.code, rsl.name, rsl.sort_order
ORDER BY rsl.sort_order NULLS LAST`, where)

	rows, err := keyLabelCount(ctx, r.db, query, args)
	if err != nil {
		return err
	}
	out.BySeverity = rows
	return nil
}

func (r *Repository) loadType(ctx context.Context, out *analyticsdomain.Summary, where string, args []any) error {
	query := fmt.Sprintf(`
SELECT COALESCE(rit.code, 'UNKNOWN'), COALESCE(rit.name, 'Unknown'), COUNT(*)
FROM incidents i
LEFT JOIN ref_incident_types rit ON rit.id = i.incident_type_id
WHERE %s
GROUP BY rit.code, rit.name
ORDER BY COUNT(*) DESC`, where)

	rows, err := keyLabelCount(ctx, r.db, query, args)
	if err != nil {
		return err
	}
	out.ByType = rows
	return nil
}

func (r *Repository) loadDistricts(ctx context.Context, out *analyticsdomain.Summary, where string, args []any) error {
	query := fmt.Sprintf(`
SELECT
    COALESCE(rd.id::text, ''),
    COALESCE(rd.name, 'Unassigned'),
    COUNT(*),
    COUNT(*) FILTER (WHERE i.referring_facility_id IS NOT NULL),
    COUNT(*) FILTER (WHERE i.receiving_facility_id IS NOT NULL),
    COUNT(*) FILTER (WHERE rpl.code = 'RED'),
    COUNT(*) FILTER (WHERE i.status = 'COMPLETED')
FROM incidents i
LEFT JOIN ref_districts rd ON rd.id = i.district_id
LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
WHERE %s
GROUP BY rd.id, rd.name
ORDER BY COUNT(*) DESC`, where)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	districts := []analyticsdomain.DistrictRow{}
	for rows.Next() {
		var d analyticsdomain.DistrictRow
		if err := rows.Scan(&d.DistrictID, &d.District, &d.Total, &d.Referrals, &d.Transfers, &d.Critical, &d.Completed); err != nil {
			return err
		}
		districts = append(districts, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	out.ByDistrict = districts
	return nil
}

func (r *Repository) loadPatient(ctx context.Context, out *analyticsdomain.Summary, where string, args []any) error {
	// Transported patients — derived from incident status.
	transportedQuery := fmt.Sprintf(`SELECT COUNT(*) FROM incidents i WHERE %s AND %s`, where, transportedStatusFilter)
	if err := r.db.QueryRow(ctx, transportedQuery, args...).Scan(&out.PatientReport.Transported); err != nil {
		return err
	}

	sexQuery := fmt.Sprintf(`
SELECT COALESCE(NULLIF(i.patient_sex, ''), 'UNKNOWN'), COUNT(*)
FROM incidents i
WHERE %s
GROUP BY 1
ORDER BY COUNT(*) DESC`, where)
	bySex, err := keyCount(ctx, r.db, sexQuery, args, true)
	if err != nil {
		return err
	}
	out.PatientReport.BySex = bySex

	ageQuery := fmt.Sprintf(`
SELECT COALESCE(NULLIF(i.patient_age_group, ''), 'Unknown'), COUNT(*)
FROM incidents i
WHERE %s
GROUP BY 1
ORDER BY COUNT(*) DESC`, where)
	byAge, err := keyCount(ctx, r.db, ageQuery, args, false)
	if err != nil {
		return err
	}
	out.PatientReport.ByAgeGroup = byAge

	// Latest recorded outcome per incident, bucketed by outcome status.
	outcomeQuery := fmt.Sprintf(`
SELECT latest.outcome_status, COUNT(*)
FROM (
    SELECT DISTINCT ON (f.incident_id) f.incident_id, f.outcome_status
    FROM incident_feedback f
    JOIN incidents i ON i.id = f.incident_id
    WHERE %s
    ORDER BY f.incident_id, f.created_at DESC
) latest
GROUP BY latest.outcome_status
ORDER BY COUNT(*) DESC`, where)
	outcomes, err := keyCount(ctx, r.db, outcomeQuery, args, true)
	if err != nil {
		return err
	}
	out.PatientReport.Outcomes = outcomes

	return nil
}

// keyCount runs a (key, count) query. When humanize is true the key is
// title-cased into the label (e.g. AT_SCENE -> "At Scene").
func keyCount(ctx context.Context, db *pgxpool.Pool, query string, args []any, humanize bool) ([]analyticsdomain.Breakdown, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []analyticsdomain.Breakdown{}
	for rows.Next() {
		var b analyticsdomain.Breakdown
		if err := rows.Scan(&b.Key, &b.Count); err != nil {
			return nil, err
		}
		if humanize {
			b.Label = humanizeCode(b.Key)
		} else {
			b.Label = b.Key
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// keyLabelCount runs a (key, label, count) query verbatim.
func keyLabelCount(ctx context.Context, db *pgxpool.Pool, query string, args []any) ([]analyticsdomain.Breakdown, error) {
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
		out = append(out, b)
	}
	return out, rows.Err()
}

// humanizeCode turns an UPPER_SNAKE code into a "Title Case" label.
func humanizeCode(code string) string {
	if code == "" {
		return "Unknown"
	}
	parts := strings.FieldsFunc(code, func(r rune) bool {
		return r == '_' || r == '-' || r == ' '
	})
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}
