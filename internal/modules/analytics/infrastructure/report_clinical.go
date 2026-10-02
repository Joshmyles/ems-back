package infrastructure

import (
	"context"
	"fmt"
	"strings"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// clinicalCTE extends the lifecycle CTE with each incident's latest triage
// answer per question (resp) and the numeric reading parsed from free-text
// vitals (num). Vitals are typed by hand ("120/80mmhg", "98% on 5L O2",
// "Afebrile"), so the first number is extracted and range-checked later.
func clinicalCTE(where string) string {
	return "WITH " + lifecycleCTE(where) + `,
resp AS (
    SELECT DISTINCT ON (r.incident_id, r.question_code)
        r.incident_id, r.question_code, r.response_value_bool,
        NULLIF(TRIM(r.response_value_text), '') AS txt
    FROM incident_triage_responses r
    JOIN base b ON b.id = r.incident_id
    ORDER BY r.incident_id, r.question_code, r.created_at DESC
),
num AS (
    SELECT incident_id, question_code, txt,
        CASE WHEN question_code = 'BLOOD_PRESSURE'
             THEN substring(txt FROM '(\d{2,3})\s*/\s*\d{2,3}')::numeric
             ELSE substring(txt FROM '(\d{1,3}(?:\.\d+)?)')::numeric END AS v,
        CASE WHEN question_code = 'BLOOD_PRESSURE'
             THEN substring(txt FROM '\d{2,3}\s*/\s*(\d{2,3})')::numeric END AS v2
    FROM resp WHERE txt IS NOT NULL
)`
}

type vitalBandSpec struct {
	label, tone, cond string
}

type vitalSpec struct {
	key, code, label, unit string
	adultOnly              bool
	plausible              string
	bands                  []vitalBandSpec
}

// Reference bands are deliberately simple adult thresholds; pulse and
// respiratory rate are restricted to adolescents and adults because neonatal
// norms differ too much to share a scale.
var vitalSpecs = []vitalSpec{
	{
		key: "spo2", code: "OXYGEN_SATURATION", label: "Oxygen saturation (SpO₂)", unit: "%",
		plausible: "v BETWEEN 50 AND 100",
		bands: []vitalBandSpec{
			{"Hypoxaemia (< 90%)", "critical", "v < 90"},
			{"Low (90–93%)", "warning", "v >= 90 AND v < 94"},
			{"Normal (≥ 94%)", "good", "v >= 94"},
		},
	},
	{
		key: "sbp", code: "BLOOD_PRESSURE", label: "Systolic blood pressure", unit: "mmHg",
		plausible: "v BETWEEN 50 AND 260",
		bands: []vitalBandSpec{
			{"Hypotension (< 90)", "critical", "v < 90"},
			{"Normal (90–139)", "good", "v >= 90 AND v < 140"},
			{"Hypertensive (140–159)", "serious", "v >= 140 AND v < 160"},
			{"Severe (≥ 160)", "critical", "v >= 160"},
		},
	},
	{
		key: "pulse", code: "PULSE_RATE", label: "Pulse rate (adults & adolescents)", unit: "bpm",
		adultOnly: true, plausible: "v BETWEEN 20 AND 250",
		bands: []vitalBandSpec{
			{"Bradycardia (< 60)", "warning", "v < 60"},
			{"Normal (60–100)", "good", "v >= 60 AND v <= 100"},
			{"Tachycardia (101–130)", "serious", "v > 100 AND v <= 130"},
			{"Severe tachycardia (> 130)", "critical", "v > 130"},
		},
	},
	{
		key: "temp", code: "BODY_TEMPERATURE", label: "Body temperature", unit: "°C",
		plausible: "v BETWEEN 30 AND 45",
		bands: []vitalBandSpec{
			{"Hypothermia (< 35.5)", "critical", "v < 35.5"},
			{"Normal (35.5–37.4)", "good", "v >= 35.5 AND v < 37.5"},
			{"Low-grade fever (37.5–37.9)", "warning", "v >= 37.5 AND v < 38"},
			{"Fever (≥ 38)", "serious", "v >= 38"},
		},
	},
	{
		key: "rr", code: "RESPIRATORY_RATE", label: "Respiratory rate (adults & adolescents)", unit: "/min",
		adultOnly: true, plausible: "v BETWEEN 4 AND 80",
		bands: []vitalBandSpec{
			{"Bradypnoea (< 12)", "warning", "v < 12"},
			{"Normal (12–20)", "good", "v >= 12 AND v <= 20"},
			{"Tachypnoea (21–29)", "serious", "v > 20 AND v < 30"},
			{"Severe (≥ 30)", "critical", "v >= 30"},
		},
	},
}

var redFlagSpecs = []struct{ key, code, label, cond string }{
	{"unconscious", "IS_PATIENT_CONSCIOUS", "Unconscious", "response_value_bool = FALSE"},
	{"not_breathing", "IS_PATIENT_BREATHING", "Not breathing", "response_value_bool = FALSE"},
	{"severe_bleeding", "HAS_SEVERE_BLEEDING", "Severe bleeding", "response_value_bool = TRUE"},
	{"cannot_speak", "CAN_PATIENT_SPEAK", "Cannot speak", "response_value_bool = FALSE"},
}

var scoreHistogram = []analyticsdomain.HistogramBucket{
	{Label: "0", Min: 0, Max: 1},
	{Label: "1–20", Min: 1, Max: 21},
	{Label: "21–50", Min: 21, Max: 51},
	{Label: "51–100", Min: 51, Max: 101},
	{Label: "101–150", Min: 101, Max: 151},
	{Label: "> 150", Min: 151, Max: -1},
}

const adultAgeSQL = `(` + ageGroupSQL + `) IN ('ADOLESCENT','ADULT','ELDERLY')`

// Clinical profiles the patients behind the incidents: triage red flags, vital
// signs, the age/sex pyramid, case mix and the maternal & neonatal burden.
func (r *Repository) Clinical(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Clinical, error) {
	out := analyticsdomain.Clinical{}
	a := &sqlArgs{}
	cte := clinicalCTE(incidentWindow(a, f, f.From, f.To))

	if err := r.db.QueryRow(ctx, cte+`
SELECT COUNT(*),
    COUNT(*) FILTER (WHERE has_triage),
    COUNT(*) FILTER (WHERE auto_dispatch_eligible),
    (SELECT COUNT(*) FROM resp WHERE question_code = 'OXYGEN_SATURATION'
        AND txt ~* '(o2|oxygen|cpap|l ?/ ?min|l per min|litre|liter)' AND txt !~* 'room air')
FROM lc`, a.values...).Scan(&out.Incidents, &out.Triaged, &out.AutoDispatch, &out.OnOxygen); err != nil {
		return out, fmt.Errorf("clinical summary: %w", err)
	}

	for _, rf := range redFlagSpecs {
		flag := analyticsdomain.RedFlag{Key: rf.key, Label: rf.label}
		if err := r.db.QueryRow(ctx, cte+fmt.Sprintf(`
SELECT COUNT(*) FILTER (WHERE %s), COUNT(response_value_bool)
FROM resp WHERE question_code = '%s'`, rf.cond, rf.code), a.values...).Scan(&flag.Positive, &flag.Answered); err != nil {
			return out, fmt.Errorf("red flag %s: %w", rf.key, err)
		}
		out.RedFlags = append(out.RedFlags, flag)
	}

	for _, vs := range vitalSpecs {
		v, err := r.vital(ctx, cte, vs, a.values)
		if err != nil {
			return out, fmt.Errorf("vital %s: %w", vs.key, err)
		}
		out.Vitals = append(out.Vitals, v)
	}

	// Age/sex pyramid on the normalised age bands.
	pyr := map[string]*analyticsdomain.PyramidRow{}
	rows, err := r.db.Query(ctx, cte+fmt.Sprintf(`
SELECT %s AS age, UPPER(COALESCE(NULLIF(TRIM(i.patient_sex), ''), 'UNKNOWN')), COUNT(*)
FROM base i GROUP BY 1, 2`, ageGroupSQL), a.values...)
	if err != nil {
		return out, fmt.Errorf("pyramid: %w", err)
	}
	for rows.Next() {
		var age, sex string
		var n int64
		if err := rows.Scan(&age, &sex, &n); err != nil {
			rows.Close()
			return out, err
		}
		row, ok := pyr[age]
		if !ok {
			row = &analyticsdomain.PyramidRow{AgeGroup: age}
			pyr[age] = row
		}
		switch sex {
		case "MALE", "M":
			row.Male += n
		case "FEMALE", "F":
			row.Female += n
		default:
			row.Unknown += n
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}
	for _, g := range ageGroupOrder {
		row, ok := pyr[g.Key]
		if !ok {
			row = &analyticsdomain.PyramidRow{}
		}
		row.AgeGroup = g.Label
		out.Pyramid = append(out.Pyramid, *row)
	}

	if out.TypePriority, err = queryHeat(ctx, r.db, cte+`
SELECT prio, COALESCE(rit.name, 'Unknown'), COUNT(*)::float8
FROM lc LEFT JOIN ref_incident_types rit ON rit.id = lc.incident_type_id
GROUP BY 1, 2`, a.values); err != nil {
		return out, fmt.Errorf("type priority: %w", err)
	}

	typeRows, err := r.db.Query(ctx, cte+`
SELECT COALESCE(rit.code, 'UNKNOWN'), COALESCE(rit.name, 'Unknown'), COUNT(*),
    COUNT(*) FILTER (WHERE prio = 'RED'),
    COUNT(*) FILTER (WHERE status = 'COMPLETED'),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY m_dispatch)
FROM lc LEFT JOIN ref_incident_types rit ON rit.id = lc.incident_type_id
GROUP BY 1, 2 ORDER BY 3 DESC`, a.values...)
	if err != nil {
		return out, fmt.Errorf("by type: %w", err)
	}
	for typeRows.Next() {
		var t analyticsdomain.TypeRow
		if err := typeRows.Scan(&t.Key, &t.Label, &t.Count, &t.Critical, &t.Completed, &t.MedianDispatch); err != nil {
			typeRows.Close()
			return out, err
		}
		out.ByType = append(out.ByType, t)
	}
	typeRows.Close()
	if err := typeRows.Err(); err != nil {
		return out, err
	}

	scols := []string{}
	for _, b := range scoreHistogram {
		if b.Max < 0 {
			scols = append(scols, fmt.Sprintf("COUNT(*) FILTER (WHERE total_score >= %d)", b.Min))
		} else {
			scols = append(scols, fmt.Sprintf("COUNT(*) FILTER (WHERE total_score >= %d AND total_score < %d)", b.Min, b.Max))
		}
	}
	scounts := make([]int64, len(scoreHistogram))
	sdest := []any{}
	for i := range scounts {
		sdest = append(sdest, &scounts[i])
	}
	if err := r.db.QueryRow(ctx, cte+"\nSELECT "+strings.Join(scols, ", ")+" FROM lc WHERE has_triage", a.values...).Scan(sdest...); err != nil {
		return out, fmt.Errorf("score histogram: %w", err)
	}
	for i, b := range scoreHistogram {
		b.Count = scounts[i]
		out.ScoreHistogram = append(out.ScoreHistogram, b)
	}

	m := &out.MNH
	if err := r.db.QueryRow(ctx, cte+`,
mnh AS (
    SELECT lc.*, rit.code AS type_code
    FROM lc JOIN ref_incident_types rit ON rit.id = lc.incident_type_id
    WHERE rit.code IN ('MATERNAL_EMERGENCY','NEONATAL_EMERGENCY')
)
SELECT
    COUNT(*) FILTER (WHERE type_code = 'MATERNAL_EMERGENCY'),
    COUNT(*) FILTER (WHERE type_code = 'NEONATAL_EMERGENCY'),
    COUNT(*) FILTER (WHERE prio = 'RED'),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY m_dispatch),
    (SELECT COUNT(*) FROM num JOIN mnh ON mnh.id = num.incident_id
        WHERE mnh.type_code = 'MATERNAL_EMERGENCY' AND num.question_code = 'BLOOD_PRESSURE'
          AND num.v BETWEEN 50 AND 260
          AND (num.v >= 140 OR num.v2 >= 90)),
    (SELECT COUNT(*) FROM num JOIN mnh ON mnh.id = num.incident_id
        WHERE mnh.type_code = 'MATERNAL_EMERGENCY' AND num.question_code = 'BLOOD_PRESSURE'
          AND num.v BETWEEN 50 AND 260
          AND (num.v >= 160 OR num.v2 >= 110)),
    (SELECT COUNT(*) FROM num JOIN mnh ON mnh.id = num.incident_id
        WHERE mnh.type_code = 'MATERNAL_EMERGENCY' AND num.question_code = 'BLOOD_PRESSURE'
          AND num.v BETWEEN 50 AND 260),
    (SELECT COUNT(*) FROM num JOIN mnh ON mnh.id = num.incident_id
        WHERE mnh.type_code = 'NEONATAL_EMERGENCY' AND num.question_code = 'BODY_TEMPERATURE'
          AND num.v BETWEEN 30 AND 45 AND num.v < 36.5),
    (SELECT COUNT(*) FROM num JOIN mnh ON mnh.id = num.incident_id
        WHERE mnh.type_code = 'NEONATAL_EMERGENCY' AND num.question_code = 'BODY_TEMPERATURE'
          AND num.v BETWEEN 30 AND 45)
FROM mnh`, a.values...).Scan(&m.Maternal, &m.Neonatal, &m.Critical, &m.MedianDispatch,
		&m.HypertensiveMaternal, &m.SevereHypertension, &m.MaternalWithBP,
		&m.NeonatalHypothermia, &m.NeonatalWithTemp); err != nil {
		return out, fmt.Errorf("mnh: %w", err)
	}
	m.Share = ratio(m.Maternal+m.Neonatal, out.Incidents)

	if out.Outcomes, err = queryBreakdowns(ctx, r.db, cte+`
SELECT latest.outcome_status, '', COUNT(*)
FROM (
    SELECT DISTINCT ON (fb.incident_id) fb.incident_id, fb.outcome_status
    FROM incident_feedback fb JOIN base b ON b.id = fb.incident_id
    ORDER BY fb.incident_id, fb.created_at DESC
) latest
GROUP BY 1 ORDER BY 3 DESC`, a.values); err != nil {
		return out, fmt.Errorf("outcomes: %w", err)
	}

	return out, nil
}

func (r *Repository) vital(ctx context.Context, cte string, vs vitalSpec, args []any) (analyticsdomain.VitalSummary, error) {
	out := analyticsdomain.VitalSummary{Key: vs.key, Label: vs.label, Unit: vs.unit}
	scope := fmt.Sprintf("n.question_code = '%s'", vs.code)
	if vs.adultOnly {
		scope += " AND " + adultAgeSQL
	}
	cols := []string{
		"COUNT(*)",
		fmt.Sprintf("COUNT(*) FILTER (WHERE %s)", vs.plausible),
		fmt.Sprintf("percentile_cont(0.5) WITHIN GROUP (ORDER BY v) FILTER (WHERE %s)", vs.plausible),
	}
	for _, b := range vs.bands {
		cols = append(cols, fmt.Sprintf("COUNT(*) FILTER (WHERE %s AND %s)", vs.plausible, b.cond))
	}
	counts := make([]int64, len(vs.bands))
	dest := []any{&out.Answered, &out.Parsed, &out.Median}
	for i := range counts {
		dest = append(dest, &counts[i])
	}
	query := cte + fmt.Sprintf(`
SELECT %s
FROM num n JOIN base i ON i.id = n.incident_id
WHERE %s`, strings.Join(cols, ", "), scope)
	if err := r.db.QueryRow(ctx, query, args...).Scan(dest...); err != nil {
		return out, err
	}
	for i, b := range vs.bands {
		out.Bands = append(out.Bands, analyticsdomain.VitalBand{Label: b.label, Tone: b.tone, Count: counts[i]})
	}
	return out, nil
}
