package infrastructure

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// qualityCTE adds, per incident in the window, the extra columns the
// completeness checks need on top of the lifecycle.
func qualityCTE(where string) string {
	return "WITH " + lifecycleCTE(where) + fmt.Sprintf(`,
fb AS (SELECT DISTINCT incident_id FROM incident_feedback),
spo AS (
    SELECT DISTINCT r.incident_id
    FROM incident_triage_responses r JOIN base b ON b.id = r.incident_id
    WHERE r.question_code = 'OXYGEN_SATURATION'
      AND substring(r.response_value_text FROM '(\d{1,3})')::int BETWEEN 50 AND 100
),
x AS (
    SELECT lc.*, b.severity_level_id, b.patient_sex, b.patient_age_group,
        b.verification_status, b.caller_phone, rit.code AS type_code,
        (fb.incident_id IS NOT NULL) AS has_feedback,
        (spo.incident_id IS NOT NULL) AS has_spo2,
        COALESCE(%s AND NOT (lc.latitude = 0 AND lc.longitude = 0), FALSE) AS gps_ok
    FROM lc
    JOIN base b ON b.id = lc.id
    LEFT JOIN ref_incident_types rit ON rit.id = lc.incident_type_id
    LEFT JOIN fb ON fb.incident_id = lc.id
    LEFT JOIN spo ON spo.incident_id = lc.id
)`, validGPS("lc"))
}

type fieldSpec struct {
	key, label, group string
	target, weight    float64
	filled            string // count expression over x
	total             string // denominator expression over x
	recommendation    string
}

var fieldSpecs = []fieldSpec{
	{"priority", "Triage priority assigned", "Classification", 1.0, 3,
		"COUNT(*) FILTER (WHERE prio <> 'UNASSIGNED')", "COUNT(*)",
		"Make priority mandatory before an incident can be dispatched."},
	{"type", "Incident type classified", "Classification", 0.95, 2,
		"COUNT(*) FILTER (WHERE type_code IS NOT NULL AND type_code <> 'UNCLASSIFIED')", "COUNT(*)",
		"Prompt call-takers and the mobile app to pick a specific type instead of 'Unclassified'."},
	{"severity", "Severity level recorded", "Classification", 0.9, 1,
		"COUNT(severity_level_id)", "COUNT(*)",
		"Severity is never captured — add it to the intake forms or retire the field."},
	{"district", "District recorded", "Location", 1.0, 2,
		"COUNT(district_id)", "COUNT(*)",
		"Default the district from the referring facility when the reporter omits it."},
	{"gps", "Valid GPS coordinates", "Location", 0.9, 2,
		"COUNT(*) FILTER (WHERE gps_ok)", "COUNT(*)",
		"The mobile app sends 0,0 when location is unavailable — send null instead and retry the GPS fix."},
	{"sex", "Patient sex recorded", "Patient", 0.95, 1,
		"COUNT(*) FILTER (WHERE UPPER(patient_sex) IN ('MALE','FEMALE','M','F'))", "COUNT(*)",
		"Require patient sex at intake; allow 'Unknown' only for unconscious patients."},
	{"age", "Patient age group recorded", "Patient", 0.95, 1,
		"COUNT(*) FILTER (WHERE (" + qualityAgeSQL + ") <> 'UNKNOWN')", "COUNT(*)",
		"Standardise one age-group vocabulary across the web form and the mobile app."},
	{"triage", "Triage questionnaire completed", "Clinical", 1.0, 3,
		"COUNT(*) FILTER (WHERE has_triage)", "COUNT(*)",
		"Run triage on every incident; it drives priority and auto-dispatch eligibility."},
	{"spo2", "SpO₂ captured as a number", "Clinical", 0.8, 2,
		"COUNT(*) FILTER (WHERE has_spo2)", "COUNT(*)",
		"Capture vitals in numeric fields (with an 'on oxygen' checkbox) instead of free text."},
	{"origin", "Referring facility recorded", "Referral", 0.8, 1,
		"COUNT(referring_facility_id)", "COUNT(*)",
		"Pre-fill the referring facility from the reporting user's facility assignment."},
	{"destination", "Receiving facility recorded", "Referral", 0.9, 2,
		"COUNT(receiving_facility_id)", "COUNT(*)",
		"Require a receiving facility before dispatch so the destination can be alerted."},
	{"caller", "Caller phone recorded", "Workflow", 0.95, 1,
		"COUNT(*) FILTER (WHERE NULLIF(TRIM(caller_phone), '') IS NOT NULL)", "COUNT(*)",
		"Keep caller phone mandatory so dispatch can call back."},
	{"verification", "Incident verified", "Workflow", 0.9, 1,
		"COUNT(*) FILTER (WHERE verification_status <> 'PENDING')", "COUNT(*)",
		"Verification is never performed — either embed it in the dispatch workflow or drop the step."},
	{"milestones", "Crew milestones recorded (arrived at scene)", "Workflow", 0.8, 2,
		"COUNT(*) FILTER (WHERE t_assigned IS NOT NULL AND t_scene IS NOT NULL)", "COUNT(t_assigned)",
		"Crews rarely update status in the app; add one-tap En route / On scene / Loaded buttons and audit them."},
	{"feedback", "Receiving-facility outcome feedback", "Workflow", 0.8, 3,
		"COUNT(*) FILTER (WHERE status = 'COMPLETED' AND has_feedback)", "COUNT(*) FILTER (WHERE status = 'COMPLETED')",
		"Notify the receiving facility focal person to submit the patient outcome after every completed transfer."},
}

// qualityAgeSQL is ageGroupSQL over the x alias.
var qualityAgeSQL = strings.ReplaceAll(ageGroupSQL, "i.", "x.")

// DataQuality scores how complete and consistent the operational data is, and
// how widely the system is being used.
func (r *Repository) DataQuality(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.DataQuality, error) {
	out := analyticsdomain.DataQuality{}
	a := &sqlArgs{}
	cte := qualityCTE(incidentWindow(a, f, f.From, f.To))

	// Field completeness.
	cols := []string{"COUNT(*)"}
	for _, fs := range fieldSpecs {
		cols = append(cols, fs.filled, fs.total)
	}
	vals := make([]int64, 1+2*len(fieldSpecs))
	dest := []any{}
	for i := range vals {
		dest = append(dest, &vals[i])
	}
	query := cte + "\nSELECT " + strings.Join(cols, ",\n    ") + " FROM x"
	if err := r.db.QueryRow(ctx, query, a.values...).Scan(dest...); err != nil {
		return out, fmt.Errorf("completeness: %w", err)
	}
	out.Incidents = vals[0]
	for i, fs := range fieldSpecs {
		out.Fields = append(out.Fields, analyticsdomain.FieldQuality{
			Key: fs.key, Label: fs.label, Group: fs.group,
			Filled: vals[1+2*i], Total: vals[2+2*i],
			Target: fs.target, Weight: fs.weight, Recommendation: fs.recommendation,
		})
	}

	issues, err := r.qualityIssues(ctx, f, cte, a.values, out.Incidents)
	if err != nil {
		return out, err
	}
	out.Issues = issues

	// Weekly/monthly trend of the key capture rates.
	trendRows, err := r.db.Query(ctx, cte+fmt.Sprintf(`
SELECT %s, COUNT(*),
    AVG((prio <> 'UNASSIGNED')::int)::float8,
    AVG((type_code IS NOT NULL AND type_code <> 'UNCLASSIFIED')::int)::float8,
    AVG(has_triage::int)::float8,
    AVG(gps_ok::int)::float8,
    AVG((receiving_facility_id IS NOT NULL)::int)::float8,
    AVG(has_feedback::int) FILTER (WHERE status = 'COMPLETED')::float8
FROM x GROUP BY 1 ORDER BY 1`, bucketSQL("reported_at", f.Granularity)), a.values...)
	if err != nil {
		return out, fmt.Errorf("quality trend: %w", err)
	}
	for trendRows.Next() {
		var p analyticsdomain.QualityTrendPoint
		if err := trendRows.Scan(&p.Bucket, &p.Total, &p.Priority, &p.Classified, &p.Triage, &p.GPS, &p.Facility, &p.Feedback); err != nil {
			trendRows.Close()
			return out, err
		}
		out.Trend = append(out.Trend, p)
	}
	trendRows.Close()
	if err := trendRows.Err(); err != nil {
		return out, err
	}

	chRows, err := r.db.Query(ctx, cte+`
SELECT source_channel, COUNT(*),
    (AVG((prio <> 'UNASSIGNED')::int) + AVG((type_code IS NOT NULL AND type_code <> 'UNCLASSIFIED')::int)
     + AVG(has_triage::int) + AVG(gps_ok::int) + AVG((district_id IS NOT NULL)::int)
     + AVG((receiving_facility_id IS NOT NULL)::int))::float8 / 6.0
FROM x GROUP BY 1 ORDER BY 2 DESC`, a.values...)
	if err != nil {
		return out, fmt.Errorf("quality by channel: %w", err)
	}
	for chRows.Next() {
		var c analyticsdomain.ChannelQuality
		if err := chRows.Scan(&c.Channel, &c.Total, &c.Score); err != nil {
			chRows.Close()
			return out, err
		}
		out.ByChannel = append(out.ByChannel, c)
	}
	chRows.Close()
	if err := chRows.Err(); err != nil {
		return out, err
	}

	if out.Adoption, err = r.adoption(ctx, f); err != nil {
		return out, fmt.Errorf("adoption: %w", err)
	}
	return out, nil
}

func (r *Repository) qualityIssues(ctx context.Context, f analyticsdomain.ReportFilters, cte string, args []any, total int64) ([]analyticsdomain.QualityIssue, error) {
	var (
		zeroGPS, webAge, appAge, mismatch, stale, regressions, orphans,
		unparseable, duplicates, noMilestones, assigned int64
	)
	staleArgs := append(append([]any{}, args...), time.Now().Add(-24*time.Hour))
	stalePos := fmt.Sprintf("$%d", len(staleArgs))
	if err := r.db.QueryRow(ctx, cte+fmt.Sprintf(`,
vit AS (
    SELECT r.incident_id, r.question_code, r.response_value_text AS txt
    FROM incident_triage_responses r JOIN base b ON b.id = r.incident_id
    WHERE r.question_code IN ('OXYGEN_SATURATION','PULSE_RATE','BODY_TEMPERATURE','RESPIRATORY_RATE','BLOOD_PRESSURE')
      AND NULLIF(TRIM(r.response_value_text), '') IS NOT NULL
)
SELECT
    (SELECT COUNT(*) FROM x WHERE latitude = 0 AND longitude = 0),
    (SELECT COUNT(*) FROM x WHERE patient_age_group IN %[1]s),
    (SELECT COUNT(*) FROM x WHERE UPPER(patient_age_group) IN ('INFANT','CHILD','ADOLESCENT','ADULT','ELDERLY')),
    (SELECT COUNT(*) FROM x JOIN ref_facilities o ON o.id = x.referring_facility_id
        WHERE x.district_id IS NOT NULL AND o.district_id IS NOT NULL AND x.district_id <> o.district_id),
    (SELECT COUNT(*) FROM x WHERE status NOT IN %[2]s AND reported_at < %[3]s),
    (SELECT COUNT(DISTINCT u.incident_id) FROM incident_updates u JOIN base b ON b.id = u.incident_id
        WHERE u.update_type = 'STATUS_CHANGE' AND u.old_value = 'COMPLETED' AND u.new_value <> 'COMPLETED'),
    (SELECT COUNT(*) FROM dispatch_assignments d JOIN base b ON b.id = d.incident_id
        WHERE d.status IN %[4]s AND b.status IN %[2]s),
    (SELECT COUNT(*) FROM vit WHERE NOT COALESCE(CASE question_code
            WHEN 'OXYGEN_SATURATION' THEN substring(txt FROM '(\d{1,3}(?:\.\d+)?)')::numeric BETWEEN 50 AND 100
            WHEN 'PULSE_RATE'        THEN substring(txt FROM '(\d{1,3}(?:\.\d+)?)')::numeric BETWEEN 20 AND 250
            WHEN 'BODY_TEMPERATURE'  THEN substring(txt FROM '(\d{1,3}(?:\.\d+)?)')::numeric BETWEEN 30 AND 45
            WHEN 'RESPIRATORY_RATE'  THEN substring(txt FROM '(\d{1,3}(?:\.\d+)?)')::numeric BETWEEN 4 AND 80
            WHEN 'BLOOD_PRESSURE'    THEN substring(txt FROM '(\d{2,3})\s*/\s*\d{2,3}')::numeric BETWEEN 50 AND 260
        END, FALSE)),
    (SELECT COUNT(DISTINCT b2.id) FROM base b1 JOIN base b2
        ON b2.id <> b1.id
       AND NULLIF(TRIM(b1.caller_phone), '') IS NOT NULL
       AND b2.caller_phone = b1.caller_phone
       AND LOWER(TRIM(COALESCE(b2.patient_name, ''))) = LOWER(TRIM(COALESCE(b1.patient_name, '')))
       AND b2.reported_at > b1.reported_at
       AND b2.reported_at <= b1.reported_at + INTERVAL '2 hours'),
    (SELECT COUNT(*) FROM x WHERE t_assigned IS NOT NULL
        AND t_accepted IS NULL AND t_departed IS NULL AND t_scene IS NULL AND t_loaded IS NULL AND t_dest IS NULL),
    (SELECT COUNT(*) FROM x WHERE t_assigned IS NOT NULL)
`, canonicalAgeSQL, terminalStatusSQL, stalePos, activeDispatchSQL), staleArgs...).Scan(
		&zeroGPS, &webAge, &appAge, &mismatch, &stale, &regressions, &orphans,
		&unparseable, &duplicates, &noMilestones, &assigned,
	); err != nil {
		return nil, fmt.Errorf("quality issues: %w", err)
	}

	// Fuel logs whose odometer went backwards or did not move.
	fa := &sqlArgs{}
	fuel := fuelCTE(fa, f)
	var odometerFaults int64
	if err := r.db.QueryRow(ctx, "WITH "+fuel+`
SELECT COUNT(DISTINCT id) FROM an WHERE code IN ('ODOMETER_ROLLBACK','NO_DISTANCE','ODOMETER_JUMP')`, fa.values...).Scan(&odometerFaults); err != nil {
		return nil, fmt.Errorf("fuel odometer issues: %w", err)
	}

	// Facilities that sent or received patients but have no usable coordinates
	// in the register, so flows cannot be mapped or distances computed.
	var unlocatedFacilities int64
	if err := r.db.QueryRow(ctx, cte+fmt.Sprintf(`
SELECT COUNT(DISTINCT rf.id)
FROM x JOIN ref_facilities rf ON rf.id IN (x.referring_facility_id, x.receiving_facility_id)
WHERE NOT COALESCE(%s, FALSE)`, validGPS("rf")), args...).Scan(&unlocatedFacilities); err != nil {
		return nil, fmt.Errorf("facility coordinates: %w", err)
	}

	issues := []analyticsdomain.QualityIssue{}
	add := func(key, label, severity, detail string, count int64) {
		if count > 0 {
			issues = append(issues, analyticsdomain.QualityIssue{Key: key, Label: label, Severity: severity, Detail: detail, Count: count})
		}
	}
	gpsSeverity := "warning"
	if ratio(zeroGPS, total) > 0.2 {
		gpsSeverity = "critical"
	}
	add("gps_placeholder", "Placeholder GPS coordinates (0, 0)", gpsSeverity,
		fmt.Sprintf("%.0f%% of incidents were saved at 0°N 0°E (Gulf of Guinea), so they cannot be mapped.", 100*ratio(zeroGPS, total)), zeroGPS)
	if webAge > 0 && appAge > 0 {
		add("age_vocabulary", "Two age-group vocabularies in use", "warning",
			fmt.Sprintf("%d incidents use web-form bands (e.g. 18-59) and %d use mobile-app labels (e.g. ADULT). Reports normalise them, but the source should be unified.", webAge, appAge),
			min64(webAge, appAge))
	}
	add("facility_coordinates", "Referral facilities without GPS coordinates", "warning",
		"These facilities sent or received patients but have no coordinates in the facility register, so referral flows cannot be mapped and transfer distances cannot be computed. Import coordinates from the national health facility registry (NHFR) using each facility's nhfr_id.",
		unlocatedFacilities)
	add("missing_milestones", "No crew status updates after assignment", "warning",
		fmt.Sprintf("%.0f%% of assigned incidents have no accepted / en route / on scene / loaded timestamp, so response times after dispatch cannot be measured.", 100*ratio(noMilestones, assigned)), noMilestones)
	add("stale_open", "Incidents open for more than 24 hours", "warning",
		"Cases still not completed, cancelled or rejected a day after being reported — close them or record why they are still active.", stale)
	add("orphan_assignments", "Live dispatches on closed incidents", "warning",
		"Dispatch assignments still active although their incident is closed; these block re-dispatch.", orphans)
	add("status_regressions", "Completed incidents re-opened", "info",
		"Incidents moved back out of COMPLETED. Verify these were intentional corrections.", regressions)
	add("possible_duplicates", "Possible duplicate reports", "info",
		"Same caller and patient reported again within 2 hours.", duplicates)
	add("district_mismatch", "Incident district differs from the referring facility's", "info",
		"The incident district does not match where the referring facility is registered.", mismatch)
	add("vitals_unparseable", "Vital signs that could not be read", "info",
		"Free-text answers such as 'Na', 'Not taken' or out-of-range values (e.g. 366.5 °C).", unparseable)
	add("fuel_odometer", "Fuel logs with faulty odometer readings", "warning",
		"Rollbacks, repeated readings or implausible jumps make fuel efficiency unverifiable.", odometerFaults)

	rank := map[string]int{"critical": 0, "warning": 1, "info": 2}
	sort.SliceStable(issues, func(i, j int) bool {
		if rank[issues[i].Severity] != rank[issues[j].Severity] {
			return rank[issues[i].Severity] < rank[issues[j].Severity]
		}
		return issues[i].Count > issues[j].Count
	})
	return issues, nil
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func (r *Repository) adoption(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Adoption, error) {
	out := analyticsdomain.Adoption{}
	if err := r.db.QueryRow(ctx, `
SELECT (SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND is_active),
       (SELECT COUNT(DISTINCT user_id) FROM auth_sessions WHERE created_at >= $1 AND created_at < $2)`,
		f.From, f.To).Scan(&out.TotalUsers, &out.ActiveUsers); err != nil {
		return out, err
	}

	rows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT %s, COUNT(DISTINCT user_id), COUNT(*)
FROM auth_sessions WHERE created_at >= $1 AND created_at < $2
GROUP BY 1 ORDER BY 1`, bucketSQL("created_at", f.Granularity)), f.From, f.To)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var p analyticsdomain.ActivityPoint
		if err := rows.Scan(&p.Bucket, &p.Users, &p.Sessions); err != nil {
			rows.Close()
			return out, err
		}
		out.Activity = append(out.Activity, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	nrows, err := r.db.Query(ctx, `
SELECT channel, COUNT(*),
    COUNT(*) FILTER (WHERE status = 'READ' OR read_at IS NOT NULL),
    COUNT(*) FILTER (WHERE status IN ('SENT','DELIVERED','READ') OR sent_at IS NOT NULL),
    COUNT(*) FILTER (WHERE status = 'PENDING'),
    COUNT(*) FILTER (WHERE status = 'FAILED')
FROM notifications WHERE created_at >= $1 AND created_at < $2
GROUP BY 1 ORDER BY 2 DESC`, f.From, f.To)
	if err != nil {
		return out, err
	}
	for nrows.Next() {
		var n analyticsdomain.NotificationRow
		if err := nrows.Scan(&n.Channel, &n.Total, &n.Read, &n.Sent, &n.Pending, &n.Failed); err != nil {
			nrows.Close()
			return out, err
		}
		out.Notifications = append(out.Notifications, n)
	}
	nrows.Close()
	if err := nrows.Err(); err != nil {
		return out, err
	}

	out.UsersByRole, err = queryBreakdowns(ctx, r.db, `
SELECT r.code, r.name, COUNT(DISTINCT ur.user_id)
FROM user_roles ur JOIN roles r ON r.id = ur.role_id JOIN users u ON u.id = ur.user_id
WHERE ur.active AND u.deleted_at IS NULL
GROUP BY 1, 2 ORDER BY 3 DESC`, nil)
	return out, err
}
