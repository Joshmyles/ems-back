package infrastructure

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	analyticsapp "dispatch/internal/modules/analytics/application"
	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

func eat(col string) string {
	return fmt.Sprintf("to_char(%s AT TIME ZONE %s, 'YYYY-MM-DD HH24:MI')", col, tzSQL)
}

const personNameSQL = `NULLIF(TRIM(CONCAT_WS(' ', %[1]s.first_name, %[1]s.last_name)), '')`

func personName(alias string) string { return fmt.Sprintf(personNameSQL, alias) }

// Export streams a register as CSV. Registers deliberately exclude caller and
// patient names and phone numbers so they can be shared for M&E.
func (r *Repository) Export(ctx context.Context, f analyticsdomain.ReportFilters, dataset string, w io.Writer) error {
	a := &sqlArgs{}
	var headers []string
	var query string

	switch dataset {
	case analyticsapp.ExportIncidents:
		headers = []string{
			"Incident number", "Reported (EAT)", "Status", "Channel", "Incident type", "Priority",
			"District", "Subcounty", "Parish", "Village", "Patient sex", "Patient age group",
			"Referring facility", "Receiving facility", "Triage score",
			"Triaged (EAT)", "Assigned (EAT)", "Accepted (EAT)", "Departed (EAT)", "At scene (EAT)",
			"Patient loaded (EAT)", "At facility (EAT)", "Completed (EAT)",
			"Minutes to dispatch", "Minutes to scene", "Minutes to completion",
			"Ambulance", "Driver", "Lead medic", "Dispatcher", "Latest outcome", "Valid GPS",
		}
		query = "WITH " + lifecycleCTE(incidentWindow(a, f, f.From, f.To)) + fmt.Sprintf(`,
fbl AS (
    SELECT DISTINCT ON (incident_id) incident_id, outcome_status
    FROM incident_feedback ORDER BY incident_id, created_at DESC
)
SELECT lc.incident_number, %s, lc.status, lc.source_channel, COALESCE(rit.name, ''), lc.prio,
    COALESCE(rd.name, ''), COALESCE(b.subcounty, ''), COALESCE(b.parish, ''), COALESCE(b.village, ''),
    COALESCE(b.patient_sex, ''), COALESCE(b.patient_age_group, ''),
    COALESCE(o.name, ''), COALESCE(dst.name, ''), lc.total_score,
    %s, %s, %s, %s, %s, %s, %s, %s,
    round(lc.m_dispatch::numeric, 1), round(lc.m_scene::numeric, 1), round(lc.m_close::numeric, 1),
    COALESCE(am.plate_number, ''), COALESCE(%s, ''), COALESCE(%s, ''), COALESCE(%s, ''),
    COALESCE(fbl.outcome_status, ''),
    CASE WHEN %s AND NOT (lc.latitude = 0 AND lc.longitude = 0) THEN 'yes' ELSE 'no' END
FROM lc
JOIN base b ON b.id = lc.id
LEFT JOIN ref_incident_types rit ON rit.id = lc.incident_type_id
LEFT JOIN ref_districts rd ON rd.id = lc.district_id
LEFT JOIN ref_facilities o ON o.id = lc.referring_facility_id
LEFT JOIN ref_facilities dst ON dst.id = lc.receiving_facility_id
LEFT JOIN ambulances am ON am.id = lc.ambulance_id
LEFT JOIN users drv ON drv.id = lc.driver_user_id
LEFT JOIN users med ON med.id = lc.lead_medic_user_id
LEFT JOIN users dsp ON dsp.id = lc.assigned_by_user_id
LEFT JOIN fbl ON fbl.incident_id = lc.id
ORDER BY lc.reported_at`,
			eat("lc.reported_at"),
			eat("lc.t_triaged"), eat("lc.t_assigned"), eat("lc.t_accepted"), eat("lc.t_departed"),
			eat("lc.t_scene"), eat("lc.t_loaded"), eat("lc.t_dest"), eat("lc.t_completed"),
			personName("drv"), personName("med"), personName("dsp"), validGPS("lc"))

	case analyticsapp.ExportDispatches:
		headers = []string{
			"Incident number", "Incident reported (EAT)", "Priority", "Ambulance", "Category",
			"Driver", "Lead medic", "Dispatcher", "Mode", "Status", "ETA (min)",
			"Assigned (EAT)", "Accepted (EAT)", "Departed (EAT)", "At scene (EAT)",
			"Patient loaded (EAT)", "At facility (EAT)", "Completed (EAT)", "Cancelled (EAT)", "Cancellation reason",
		}
		query = fmt.Sprintf(`
SELECT i.incident_number, %s, COALESCE(rpl.code, 'UNASSIGNED'),
    COALESCE(am.plate_number, ''), COALESCE(cat.code, ''),
    COALESCE(%s, ''), COALESCE(%s, ''), COALESCE(%s, ''),
    d.assignment_mode, d.status, d.eta_minutes,
    %s, %s, %s, %s, %s, %s, %s, %s, COALESCE(d.cancellation_reason, '')
FROM dispatch_assignments d
JOIN incidents i ON i.id = d.incident_id
LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
LEFT JOIN ambulances am ON am.id = d.ambulance_id
LEFT JOIN ref_ambulance_categories cat ON cat.id = am.category_id
LEFT JOIN users drv ON drv.id = d.driver_user_id
LEFT JOIN users med ON med.id = d.lead_medic_user_id
LEFT JOIN users dsp ON dsp.id = d.assigned_by_user_id
WHERE %s
ORDER BY d.created_at`,
			eat("i.reported_at"), personName("drv"), personName("med"), personName("dsp"),
			eat("d.assigned_at"), eat("d.accepted_at"), eat("d.departed_at"), eat("d.arrived_scene_at"),
			eat("d.patient_loaded_at"), eat("d.arrived_destination_at"), eat("d.completed_at"), eat("d.cancelled_at"),
			incidentWindow(a, f, f.From, f.To))

	case analyticsapp.ExportFuel:
		headers = []string{
			"Filled (EAT)", "Ambulance", "Fuel type", "Litres", "Cost (UGX)", "Implied UGX/L",
			"Odometer (km)", "Km since previous fill", "Km per litre", "Plausible interval",
			"Station", "Funding source", "Dispense confirmed", "Integrity flags",
		}
		query = "WITH " + fuelCTE(a, f) + fmt.Sprintf(`,
flags AS (SELECT id, string_agg(code, '; ' ORDER BY code) AS codes FROM an GROUP BY id)
SELECT %s, COALESCE(x.plate_number, ''), COALESCE(x.fuel_type, ''), x.liters, x.cost,
    round(x.implied_price::numeric, 0), x.odometer_km, x.km,
    round((x.km / NULLIF(x.liters, 0))::numeric, 2),
    CASE WHEN x.valid_interval THEN 'yes' ELSE 'no' END,
    COALESCE(x.station_name, ''), COALESCE(fs.organisation_name, ''),
    CASE WHEN x.dispense_confirmed THEN 'yes' ELSE 'no' END,
    COALESCE(flags.codes, '')
FROM x
LEFT JOIN fuel_funding_sources fs ON fs.id = x.funding_source_id
LEFT JOIN flags ON flags.id = x.id
ORDER BY x.filled_at`, eat("x.filled_at"))

	case analyticsapp.ExportReferrals:
		headers = []string{
			"Referring facility", "Referring level", "Referring district",
			"Receiving facility", "Receiving level", "Receiving district",
			"Transfers", "Critical (red)", "Median minutes to dispatch",
		}
		query = "WITH " + lifecycleCTE(incidentWindow(a, f, f.From, f.To)) + `
SELECT o.name, COALESCE(ol.name, ''), COALESCE(od.name, ''),
    d.name, COALESCE(dl.name, ''), COALESCE(dd.name, ''),
    COUNT(*), COUNT(*) FILTER (WHERE lc.prio = 'RED'),
    round((percentile_cont(0.5) WITHIN GROUP (ORDER BY lc.m_dispatch))::numeric, 1)
FROM lc
JOIN ref_facilities o ON o.id = lc.referring_facility_id
JOIN ref_facilities d ON d.id = lc.receiving_facility_id
LEFT JOIN ref_facility_levels ol ON ol.id = o.level_id
LEFT JOIN ref_facility_levels dl ON dl.id = d.level_id
LEFT JOIN ref_districts od ON od.id = o.district_id
LEFT JOIN ref_districts dd ON dd.id = d.district_id
GROUP BY o.name, ol.name, od.name, d.name, dl.name, dd.name
ORDER BY COUNT(*) DESC, o.name, d.name`

	default:
		return fmt.Errorf("%w: unknown dataset %q", analyticsapp.ErrInvalidReportQuery, dataset)
	}

	rows, err := r.db.Query(ctx, query, a.values...)
	if err != nil {
		return err
	}
	defer rows.Close()

	// UTF-8 BOM so Excel opens accented names correctly.
	if _, err := w.Write([]byte("\xEF\xBB\xBF")); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(headers); err != nil {
		return err
	}
	record := make([]string, len(headers))
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return err
		}
		for i := range record {
			record[i] = ""
			if i < len(values) {
				record[i] = csvValue(values[i])
			}
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	cw.Flush()
	return cw.Error()
}

func csvValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "yes"
		}
		return "no"
	case int16:
		return strconv.FormatInt(int64(t), 10)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case float32:
		return strconv.FormatFloat(float64(t), 'f', -1, 32)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case time.Time:
		return t.In(analyticsapp.ReportLocation).Format("2006-01-02 15:04")
	case pgtype.Numeric:
		if !t.Valid {
			return ""
		}
		f, err := t.Float64Value()
		if err != nil || !f.Valid {
			return ""
		}
		return strconv.FormatFloat(f.Float64, 'f', -1, 64)
	default:
		return fmt.Sprint(t)
	}
}
