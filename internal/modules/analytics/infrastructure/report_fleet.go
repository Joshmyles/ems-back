package infrastructure

import (
	"context"
	"fmt"
	"math"
	"sort"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// Fuel-integrity thresholds. An interval is the distance driven between two
// fills divided by the litres of the later fill ("fill-to-fill").
const (
	maxPlausibleKm  = 1500 // between fills; larger jumps are odometer typos
	minPlausibleKmL = 2.0
	maxPlausibleKmL = 25.0
	maxTankLiters   = 120.0
	priceLowFactor  = 0.6
	priceHighFactor = 1.6
)

// fuelCTE computes, for every fuel log in the window, the distance since the
// ambulance's previous fill (over its full history, so the first fill in the
// window still has a predecessor), the implied unit price, and whether the
// interval is plausible enough to count toward efficiency. It also flags each
// log against the integrity rules (an).
func fuelCTE(a *sqlArgs, f analyticsdomain.ReportFilters) string {
	windowSQL := fmt.Sprintf("filled_at >= %s AND filled_at < %s", a.add(f.From), a.add(f.To))
	if f.DistrictID != nil {
		windowSQL += " AND amb_district = " + a.add(*f.DistrictID)
	}
	return fmt.Sprintf(`
fl_all AS (
    SELECT fl.*, a.plate_number, a.district_id AS amb_district,
        fl.odometer_km - LAG(fl.odometer_km) OVER (
            PARTITION BY fl.ambulance_id ORDER BY fl.filled_at, fl.created_at) AS km,
        CASE WHEN fl.liters > 0 AND fl.cost > 0 THEN fl.cost / fl.liters END AS implied_price
    FROM fuel_logs fl LEFT JOIN ambulances a ON a.id = fl.ambulance_id
),
w AS (SELECT * FROM fl_all WHERE %[1]s),
med AS (
    SELECT COALESCE(fuel_type, '') AS ft,
        percentile_cont(0.5) WITHIN GROUP (ORDER BY implied_price) AS mp
    FROM w GROUP BY 1
),
x AS (
    SELECT w.*, med.mp,
        COALESCE(w.km > 0 AND w.km <= %[2]d
            AND w.km / NULLIF(w.liters, 0) BETWEEN %[3]f AND %[4]f, FALSE) AS valid_interval
    FROM w LEFT JOIN med ON med.ft = COALESCE(w.fuel_type, '')
),
an AS (
    SELECT x.id, x.ambulance_id, COALESCE(x.plate_number, '') AS plate, x.filled_at, x.liters,
        COALESCE(x.cost, 0) AS cost, x.odometer_km, x.km, x.implied_price, x.mp,
        chk.code, chk.severity
    FROM x CROSS JOIN LATERAL (VALUES
        ('ODOMETER_ROLLBACK', 'critical', x.km < 0),
        ('ODOMETER_JUMP',     'serious',  x.km > %[2]d),
        ('NO_DISTANCE',       'warning',  x.km = 0),
        ('HIGH_CONSUMPTION',  'serious',  x.km > 0 AND x.km <= %[2]d AND x.km / NULLIF(x.liters, 0) < %[3]f),
        ('LOW_CONSUMPTION',   'warning',  x.km > 0 AND x.km <= %[2]d AND x.km / NULLIF(x.liters, 0) > %[4]f),
        ('OVER_CAPACITY',     'serious',  x.liters > %[5]f),
        ('PRICE_OUTLIER',     'warning',  x.implied_price < %[6]f * x.mp OR x.implied_price > %[7]f * x.mp),
        ('UNCONFIRMED',       'warning',  NOT x.dispense_confirmed AND x.filled_at < now() - INTERVAL '24 hours'),
        ('MISSING_ODOMETER',  'warning',  x.odometer_km IS NULL)
    ) AS chk(code, severity, hit)
    WHERE chk.hit
)`, windowSQL, maxPlausibleKm, minPlausibleKmL, maxPlausibleKmL, maxTankLiters, priceLowFactor, priceHighFactor)
}

var anomalyLabels = map[string]string{
	"ODOMETER_ROLLBACK": "Odometer rollback",
	"ODOMETER_JUMP":     "Odometer jump",
	"NO_DISTANCE":       "Refuel with no distance",
	"HIGH_CONSUMPTION":  "Unusually high consumption",
	"LOW_CONSUMPTION":   "Implausibly low consumption",
	"OVER_CAPACITY":     "Exceeds tank capacity",
	"PRICE_OUTLIER":     "Price outlier",
	"UNCONFIRMED":       "Dispense not confirmed",
	"MISSING_ODOMETER":  "Missing odometer reading",
}

// Fleet reports ambulance utilisation, crew workload and fuel integrity.
func (r *Repository) Fleet(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.FleetPerformance, error) {
	out := analyticsdomain.FleetPerformance{}
	a := &sqlArgs{}
	incWhere := incidentWindow(a, f, f.From, f.To)
	fuel := fuelCTE(a, f)
	ambFilter := "TRUE"
	if f.DistrictID != nil {
		ambFilter = "am.district_id = " + a.add(*f.DistrictID)
	}
	cte := fmt.Sprintf(`WITH %s,
disp AS (
    SELECT d.ambulance_id, COUNT(*) AS n,
        COUNT(DISTINCT (d.created_at AT TIME ZONE %s)::date) AS days,
        MAX(d.created_at) AS last_at
    FROM dispatch_assignments d JOIN incidents i ON i.id = d.incident_id
    WHERE %s AND d.ambulance_id IS NOT NULL
    GROUP BY d.ambulance_id
),
fuel AS (
    SELECT ambulance_id, COUNT(*) AS logs,
        COALESCE(SUM(liters), 0) AS liters, COALESCE(SUM(cost), 0) AS cost,
        COALESCE(SUM(km) FILTER (WHERE valid_interval), 0) AS valid_km,
        SUM(liters) FILTER (WHERE valid_interval) AS valid_liters,
        COUNT(*) FILTER (WHERE valid_interval) AS ok,
        COUNT(km) AS checks
    FROM x GROUP BY ambulance_id
),
anc AS (SELECT ambulance_id, COUNT(*) AS n FROM an GROUP BY ambulance_id),
amb AS (
    SELECT am.* FROM ambulances am
    WHERE (am.is_active OR am.id IN (SELECT ambulance_id FROM disp) OR am.id IN (SELECT ambulance_id FROM fuel))
      AND %s
)`, fuel, tzSQL, incWhere, ambFilter)

	rows, err := r.db.Query(ctx, cte+`
SELECT amb.id::text, amb.plate_number, COALESCE(amb.code, ''), COALESCE(cat.code, ''),
    amb.status, amb.dispatch_readiness,
    COALESCE(disp.n, 0), COALESCE(disp.days, 0), disp.last_at,
    COALESCE(fuel.logs, 0), COALESCE(fuel.liters, 0)::float8, COALESCE(fuel.cost, 0)::float8,
    COALESCE(fuel.valid_km, 0)::float8,
    (fuel.valid_km / NULLIF(fuel.valid_liters, 0))::float8,
    COALESCE(fuel.ok, 0), COALESCE(fuel.checks, 0),
    COALESCE(anc.n, 0)
FROM amb
LEFT JOIN ref_ambulance_categories cat ON cat.id = amb.category_id
LEFT JOIN disp ON disp.ambulance_id = amb.id
LEFT JOIN fuel ON fuel.ambulance_id = amb.id
LEFT JOIN anc ON anc.ambulance_id = amb.id
ORDER BY COALESCE(disp.n, 0) DESC, amb.plate_number`, a.values...)
	if err != nil {
		return out, fmt.Errorf("ambulances: %w", err)
	}
	for rows.Next() {
		var row analyticsdomain.AmbulanceRow
		if err := rows.Scan(&row.ID, &row.Plate, &row.Code, &row.Category, &row.Status, &row.Readiness,
			&row.Dispatches, &row.ActiveDays, &row.LastDispatchAt,
			&row.FuelLogs, &row.Liters, &row.FuelCost, &row.ValidKm, &row.KmPerLiter,
			&row.OdometerOK, &row.OdometerChecks, &row.Anomalies); err != nil {
			rows.Close()
			return out, err
		}
		out.Ambulances = append(out.Ambulances, row)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return out, err
	}

	// Fleet totals and workload concentration.
	t := &out.Totals
	var validKm, validLiters float64
	loads := []float64{}
	for _, row := range out.Ambulances {
		t.FleetSize++
		t.Dispatches += row.Dispatches
		t.Liters += row.Liters
		t.FuelCost += row.FuelCost
		t.Anomalies += row.Anomalies
		if row.Dispatches > 0 {
			t.Active++
		}
		if row.KmPerLiter != nil && *row.KmPerLiter > 0 {
			validKm += row.ValidKm
			validLiters += row.ValidKm / *row.KmPerLiter
		}
		loads = append(loads, float64(row.Dispatches))
	}
	t.Idle = t.FleetSize - t.Active
	var checks, ok int64
	for i := range out.Ambulances {
		row := &out.Ambulances[i]
		row.Share = ratio(row.Dispatches, t.Dispatches)
		if row.Dispatches > 0 && row.FuelCost > 0 {
			row.CostPerDispatch = ptrFloat(row.FuelCost / float64(row.Dispatches))
		}
		if row.Share > t.TopShare {
			t.TopShare = row.Share
		}
		checks += row.OdometerChecks
		ok += row.OdometerOK
	}
	if t.Dispatches > 0 && t.FuelCost > 0 {
		t.CostPerDispatch = ptrFloat(t.FuelCost / float64(t.Dispatches))
	}
	if validLiters > 0 {
		t.KmPerLiter = ptrFloat(validKm / validLiters)
	}
	if checks > 0 {
		t.OdometerReliability = ptrFloat(float64(ok) / float64(checks))
	}
	t.Gini = gini(loads)

	// Integrity anomalies, newest first.
	anRows, err := r.db.Query(ctx, cte+`
SELECT an.id::text, an.plate, an.filled_at, an.liters::float8, an.cost::float8,
    an.odometer_km::bigint, an.km::bigint, an.implied_price::float8, an.mp::float8, an.code, an.severity
FROM an ORDER BY an.filled_at DESC LIMIT 500`, a.values...)
	if err != nil {
		return out, fmt.Errorf("anomalies: %w", err)
	}
	mix := map[string]int64{}
	for anRows.Next() {
		var an analyticsdomain.FuelAnomaly
		var price, median *float64
		if err := anRows.Scan(&an.LogID, &an.Ambulance, &an.FilledAt, &an.Liters, &an.Cost,
			&an.OdometerKm, &an.KmSinceLast, &price, &median, &an.Code, &an.Severity); err != nil {
			anRows.Close()
			return out, err
		}
		an.Reason = anomalyReason(an, price, median)
		mix[an.Code]++
		out.Anomalies = append(out.Anomalies, an)
	}
	anRows.Close()
	if err := anRows.Err(); err != nil {
		return out, err
	}
	for code, n := range mix {
		out.AnomalyMix = append(out.AnomalyMix, analyticsdomain.Breakdown{Key: code, Label: anomalyLabels[code], Count: n})
	}
	sort.Slice(out.AnomalyMix, func(i, j int) bool {
		if out.AnomalyMix[i].Count == out.AnomalyMix[j].Count {
			return out.AnomalyMix[i].Key < out.AnomalyMix[j].Key
		}
		return out.AnomalyMix[i].Count > out.AnomalyMix[j].Count
	})

	// Utilisation heatmap: dispatches per ambulance per bucket.
	lb := &sqlArgs{}
	lw := incidentWindow(lb, f, f.From, f.To)
	if out.Load, err = queryHeat(ctx, r.db, fmt.Sprintf(`
SELECT %s, COALESCE(am.plate_number, 'Unknown'), COUNT(*)::float8
FROM dispatch_assignments d
JOIN incidents i ON i.id = d.incident_id
LEFT JOIN ambulances am ON am.id = d.ambulance_id
WHERE %s AND d.ambulance_id IS NOT NULL
GROUP BY 1, 2`, bucketSQL("i.reported_at", f.Granularity), lw), lb.values); err != nil {
		return out, fmt.Errorf("load: %w", err)
	}

	// Crew workload: drivers and lead medics.
	cb := &sqlArgs{}
	cw := incidentWindow(cb, f, f.From, f.To)
	crewRows, err := r.db.Query(ctx, fmt.Sprintf(`
WITH c AS (
    SELECT d.driver_user_id AS uid, 'DRIVER' AS role, d.created_at
    FROM dispatch_assignments d JOIN incidents i ON i.id = d.incident_id
    WHERE %[1]s AND d.driver_user_id IS NOT NULL
    UNION ALL
    SELECT d.lead_medic_user_id, 'MEDIC', d.created_at
    FROM dispatch_assignments d JOIN incidents i ON i.id = d.incident_id
    WHERE %[1]s AND d.lead_medic_user_id IS NOT NULL
)
SELECT u.id::text,
    COALESCE(NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), ''), u.username, 'Unknown'),
    c.role, COUNT(*), MAX(c.created_at)
FROM c JOIN users u ON u.id = c.uid
GROUP BY u.id, 2, c.role
ORDER BY COUNT(*) DESC LIMIT 40`, cw), cb.values...)
	if err != nil {
		return out, fmt.Errorf("crew: %w", err)
	}
	for crewRows.Next() {
		var p analyticsdomain.PersonRow
		if err := crewRows.Scan(&p.UserID, &p.Name, &p.Role, &p.Count, &p.LastAt); err != nil {
			crewRows.Close()
			return out, err
		}
		out.Crew = append(out.Crew, p)
	}
	crewRows.Close()
	if err := crewRows.Err(); err != nil {
		return out, err
	}

	// Median implied pump price per bucket and fuel type.
	pb := &sqlArgs{}
	pw := fmt.Sprintf("fl.filled_at >= %s AND fl.filled_at < %s", pb.add(f.From), pb.add(f.To))
	if f.DistrictID != nil {
		pw += " AND am.district_id = " + pb.add(*f.DistrictID)
	}
	priceRows, err := r.db.Query(ctx, fmt.Sprintf(`
SELECT %s, COALESCE(fl.fuel_type, 'Unspecified'),
    percentile_cont(0.5) WITHIN GROUP (ORDER BY fl.cost / NULLIF(fl.liters, 0))
        FILTER (WHERE fl.cost > 0 AND fl.liters > 0),
    COALESCE(SUM(fl.liters), 0)::float8
FROM fuel_logs fl LEFT JOIN ambulances am ON am.id = fl.ambulance_id
WHERE %s GROUP BY 1, 2 ORDER BY 1, 2`, bucketSQL("fl.filled_at", f.Granularity), pw), pb.values...)
	if err != nil {
		return out, fmt.Errorf("price trend: %w", err)
	}
	for priceRows.Next() {
		var p analyticsdomain.PricePoint
		if err := priceRows.Scan(&p.Bucket, &p.FuelType, &p.UnitPrice, &p.Liters); err != nil {
			priceRows.Close()
			return out, err
		}
		out.PriceTrend = append(out.PriceTrend, p)
	}
	priceRows.Close()
	return out, priceRows.Err()
}

func anomalyReason(an analyticsdomain.FuelAnomaly, price, median *float64) string {
	km := func() int64 {
		if an.KmSinceLast == nil {
			return 0
		}
		return *an.KmSinceLast
	}
	switch an.Code {
	case "ODOMETER_ROLLBACK":
		return fmt.Sprintf("Odometer reads %d km lower than the previous fill", -km())
	case "ODOMETER_JUMP":
		return fmt.Sprintf("Odometer jumped %d km since the previous fill — likely an entry error", km())
	case "NO_DISTANCE":
		return fmt.Sprintf("%.0f L dispensed with no distance driven since the previous fill", an.Liters)
	case "HIGH_CONSUMPTION":
		return fmt.Sprintf("Only %.1f km/L over %d km — check for leakage or misuse", float64(km())/an.Liters, km())
	case "LOW_CONSUMPTION":
		return fmt.Sprintf("%.1f km/L is implausibly efficient — a fill may be missing from the log", float64(km())/an.Liters)
	case "OVER_CAPACITY":
		return fmt.Sprintf("%.0f L exceeds a typical ambulance tank (%.0f L)", an.Liters, maxTankLiters)
	case "PRICE_OUTLIER":
		if price != nil && median != nil {
			return fmt.Sprintf("Implied price UGX %s/L vs median UGX %s/L", groupThousands(*price), groupThousands(*median))
		}
		return "Implied unit price far from the median"
	case "UNCONFIRMED":
		return "The fuel station has not confirmed dispensing this log"
	case "MISSING_ODOMETER":
		return "No odometer reading recorded, so efficiency cannot be verified"
	}
	return anomalyLabels[an.Code]
}

func groupThousands(v float64) string {
	s := fmt.Sprintf("%.0f", math.Round(v))
	neg := false
	if len(s) > 0 && s[0] == '-' {
		neg, s = true, s[1:]
	}
	out := []byte{}
	for i := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	if neg {
		return "-" + string(out)
	}
	return string(out)
}

// gini measures how unevenly dispatches are spread across the fleet: 0 means
// every ambulance carries the same load, values near 1 mean one does it all.
func gini(values []float64) float64 {
	n := len(values)
	if n < 2 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	var cum, total float64
	for i, v := range sorted {
		cum += float64(i+1) * v
		total += v
	}
	if total == 0 {
		return 0
	}
	return (2*cum)/(float64(n)*total) - float64(n+1)/float64(n)
}
