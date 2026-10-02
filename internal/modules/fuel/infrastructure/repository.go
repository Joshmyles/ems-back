package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"time"

	fuelapp "dispatch/internal/modules/fuel/application"
	"dispatch/internal/modules/fuel/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ fuelapp.Repository = (*Repository)(nil)

// fuelLogColumns is the shared projection used by List and GetByID.
const fuelLogColumns = `
	fl.id,
	fl.ambulance_id,
	fl.fuel_type,
	fl.liters,
	fl.unit_cost,
	fl.cost,
	fl.odometer_km,
	fl.station_name,
	fl.filled_at,
	fl.filled_by,
	fl.notes,
	fl.public_token,
	fl.dispensed_at,
	fl.dispense_confirmed,
	fl.attendant_name,
	fl.attendant_phone,
	fl.attendant_notes,
	fl.confirmed_at,
	fl.created_at,
	fl.updated_at,
	fl.funding_source_id,
	(SELECT s.organisation_name FROM fuel_funding_sources s WHERE s.id = fl.funding_source_id),
	(SELECT a.plate_number FROM ambulances a WHERE a.id = fl.ambulance_id),
	(SELECT NULLIF(TRIM(a.code), '') FROM ambulances a WHERE a.id = fl.ambulance_id),
	(SELECT NULLIF(TRIM(CONCAT_WS(' ', u.first_name, u.last_name)), '') FROM users u WHERE u.id = fl.filled_by)`

// driverScope restricts fuel logs (aliased fl) to ambulances the user is the
// active driver of. Without a driver it returns "TRUE" and no args.
func driverScope(driverUserID *string, pos int) (string, []any) {
	if driverUserID == nil || *driverUserID == "" {
		return "TRUE", nil
	}
	return fmt.Sprintf(`EXISTS (
		SELECT 1 FROM ambulance_crew_assignments ca
		WHERE ca.ambulance_id = fl.ambulance_id
		  AND ca.driver_user_id = $%d
		  AND ca.active = TRUE
	)`, pos), []any{*driverUserID}
}

type rowScanner interface {
	Scan(dest ...any) error
}

// scanFuelLog reads a fuel log row projected via fuelLogColumns.
func scanFuelLog(row rowScanner) (domain.FuelLog, error) {
	var fl domain.FuelLog
	var fuelType, stationName, filledBy, notes *string
	var attendantName, attendantPhone, attendantNotes *string
	var fundingSourceID, fundingSourceName *string
	var dispensedAt, confirmedAt *time.Time
	var unitCost, cost *float64
	var odometerKM *int

	if err := row.Scan(
		&fl.ID,
		&fl.AmbulanceID,
		&fuelType,
		&fl.Liters,
		&unitCost,
		&cost,
		&odometerKM,
		&stationName,
		&fl.FilledAt,
		&filledBy,
		&notes,
		&fl.PublicToken,
		&dispensedAt,
		&fl.DispenseConfirmed,
		&attendantName,
		&attendantPhone,
		&attendantNotes,
		&confirmedAt,
		&fl.CreatedAt,
		&fl.UpdatedAt,
		&fundingSourceID,
		&fundingSourceName,
		&fl.AmbulancePlate,
		&fl.AmbulanceCode,
		&fl.FilledByName,
	); err != nil {
		return domain.FuelLog{}, err
	}

	fl.FuelType = fuelType
	fl.UnitCost = unitCost
	fl.Cost = cost
	fl.OdometerKM = odometerKM
	fl.StationName = stationName
	fl.FilledBy = filledBy
	fl.Notes = notes
	fl.DispensedAt = dispensedAt
	fl.AttendantName = attendantName
	fl.AttendantPhone = attendantPhone
	fl.AttendantNotes = attendantNotes
	fl.ConfirmedAt = confirmedAt
	fl.FundingSourceID = fundingSourceID
	fl.FundingSourceName = fundingSourceName
	return fl, nil
}

func (r *Repository) List(ctx context.Context, p platformdb.Pagination, driverUserID *string) ([]domain.FuelLog, int64, error) {
	allowedSorts := map[string]string{
		"created_at":  "fl.created_at",
		"filled_at":   "fl.filled_at",
		"liters":      "fl.liters",
		"cost":        "fl.cost",
		"odometer_km": "fl.odometer_km",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	pos := 1

	if scope, scopeArgs := driverScope(driverUserID, pos); len(scopeArgs) > 0 {
		where = append(where, scope)
		args = append(args, scopeArgs...)
		pos++
	}

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			COALESCE(fl.fuel_type,'') ILIKE $%[1]d OR
			COALESCE(fl.station_name,'') ILIKE $%[1]d OR
			COALESCE(fl.notes,'') ILIKE $%[1]d OR
			COALESCE(fl.attendant_name,'') ILIKE $%[1]d OR
			EXISTS (
				SELECT 1 FROM ambulances a_s
				WHERE a_s.id = fl.ambulance_id
				  AND (REPLACE(COALESCE(a_s.plate_number,''), ' ', '') ILIKE REPLACE($%[1]d, ' ', '')
				       OR COALESCE(a_s.code,'') ILIKE $%[1]d)
			)
		)`, pos))
		args = append(args, "%"+p.Search+"%")
		pos++
	}

	for k, v := range p.Filters {
		switch k {
		case "ambulance_id":
			where = append(where, fmt.Sprintf("fl.ambulance_id = $%d", pos))
			args = append(args, v)
			pos++
		case "status":
			switch strings.ToLower(v) {
			case "pending":
				where = append(where, "fl.dispense_confirmed = FALSE")
			case "confirmed":
				where = append(where, "fl.dispense_confirmed = TRUE")
			}
		case "fuel_type":
			where = append(where, fmt.Sprintf("LOWER(TRIM(COALESCE(fl.fuel_type,''))) = LOWER(TRIM($%d))", pos))
			args = append(args, v)
			pos++
		case "funding_source_id":
			where = append(where, fmt.Sprintf("fl.funding_source_id::text = LOWER($%d)", pos))
			args = append(args, v)
			pos++
		case "user_id":
			// Return fuel logs for any ambulance the user is the active driver
			// on, OR fuel logs they personally filled in. This lets a manager
			// look up a specific user's fuel activity.
			where = append(where, fmt.Sprintf(`(
				fl.filled_by = $%d OR EXISTS (
					SELECT 1 FROM ambulance_crew_assignments ca_uid
					WHERE ca_uid.ambulance_id = fl.ambulance_id
					  AND ca_uid.driver_user_id = $%d
					  AND ca_uid.active = TRUE
				)
			)`, pos, pos))
			args = append(args, v)
			pos++
		case "date_from":
			where = append(where, fmt.Sprintf("fl.filled_at >= $%d", pos))
			args = append(args, v)
			pos++
		case "date_to":
			where = append(where, fmt.Sprintf("fl.filled_at <= $%d", pos))
			args = append(args, v)
			pos++
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, fmt.Sprintf(`SELECT COUNT(1) FROM fuel_logs fl %s`, whereSQL), args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)

	q := fmt.Sprintf(`
SELECT%s
FROM fuel_logs fl
%s
%s
LIMIT $%d OFFSET $%d
`, fuelLogColumns, whereSQL, orderBy, pos, pos+1)

	rows, err := r.db.Query(ctx, q, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.FuelLog, 0)
	for rows.Next() {
		fl, err := scanFuelLog(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, fl)
	}
	return items, total, rows.Err()
}

// Summarize aggregates fuel logs for the fuel board. Every query applies the
// same driver scope as List so a driver only sees their own ambulances.
func (r *Repository) Summarize(ctx context.Context, driverUserID *string) (domain.FuelLogSummary, error) {
	out := domain.FuelLogSummary{
		FuelTypes:   make([]domain.FuelTypeCount, 0),
		Weekly:      make([]domain.FuelWeek, 0, 12),
		FundingBurn: make([]domain.FundingBurn, 0),
	}
	scope, args := driverScope(driverUserID, 1)

	const recent = `fl.filled_at >= now() - interval '30 days'`
	const previous = `fl.filled_at >= now() - interval '60 days' AND fl.filled_at < now() - interval '30 days'`
	if err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(1),
			COUNT(1) FILTER (WHERE NOT fl.dispense_confirmed),
			COUNT(1) FILTER (WHERE fl.dispense_confirmed),
			COUNT(1) FILTER (WHERE NOT fl.dispense_confirmed AND fl.filled_at < now() - interval '24 hours'),
			MIN(fl.filled_at) FILTER (WHERE NOT fl.dispense_confirmed),
			COUNT(1) FILTER (WHERE `+recent+`),
			COALESCE(SUM(fl.liters) FILTER (WHERE `+recent+`), 0)::float8,
			COALESCE(SUM(fl.cost) FILTER (WHERE `+recent+`), 0)::float8,
			COALESCE(SUM(fl.liters) FILTER (WHERE `+recent+` AND fl.cost IS NOT NULL), 0)::float8,
			COUNT(1) FILTER (WHERE `+previous+`),
			COALESCE(SUM(fl.liters) FILTER (WHERE `+previous+`), 0)::float8,
			COALESCE(SUM(fl.cost) FILTER (WHERE `+previous+`), 0)::float8,
			COALESCE(SUM(fl.liters) FILTER (WHERE `+previous+` AND fl.cost IS NOT NULL), 0)::float8
		FROM fuel_logs fl
		WHERE `+scope, args...).Scan(
		&out.Total, &out.Pending, &out.Confirmed, &out.PendingOverdue, &out.OldestPendingAt,
		&out.Recent.Logs, &out.Recent.Liters, &out.Recent.Cost, &out.Recent.PricedLiters,
		&out.Previous.Logs, &out.Previous.Liters, &out.Previous.Cost, &out.Previous.PricedLiters,
	); err != nil {
		return out, err
	}

	// Fuel types are free text, so group case-insensitively; blanks are skipped.
	rows, err := r.db.Query(ctx, `
		SELECT INITCAP(LOWER(TRIM(fl.fuel_type))), COUNT(1)
		FROM fuel_logs fl
		WHERE NULLIF(TRIM(fl.fuel_type), '') IS NOT NULL AND `+scope+`
		GROUP BY 1
		ORDER BY 2 DESC, 1`, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var ft domain.FuelTypeCount
		if err := rows.Scan(&ft.FuelType, &ft.Count); err != nil {
			return out, err
		}
		out.FuelTypes = append(out.FuelTypes, ft)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	// The week series is generated so weeks with no logs still appear.
	rows, err = r.db.Query(ctx, `
		SELECT w.week_start, COUNT(fl.id),
			COALESCE(SUM(fl.liters), 0)::float8,
			COALESCE(SUM(fl.cost), 0)::float8
		FROM generate_series(
			date_trunc('week', now()) - interval '11 weeks',
			date_trunc('week', now()),
			interval '1 week'
		) AS w(week_start)
		LEFT JOIN fuel_logs fl
			ON fl.filled_at >= w.week_start
			AND fl.filled_at < w.week_start + interval '1 week'
			AND `+scope+`
		GROUP BY w.week_start
		ORDER BY w.week_start`, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var wk domain.FuelWeek
		if err := rows.Scan(&wk.WeekStart, &wk.Logs, &wk.Liters, &wk.Cost); err != nil {
			return out, err
		}
		out.Weekly = append(out.Weekly, wk)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	rows, err = r.db.Query(ctx, `
		SELECT fl.funding_source_id::text, COALESCE(SUM(fl.cost), 0)::float8
		FROM fuel_logs fl
		WHERE fl.funding_source_id IS NOT NULL AND `+recent+` AND `+scope+`
		GROUP BY fl.funding_source_id
		ORDER BY 2 DESC`, args...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var fb domain.FundingBurn
		if err := rows.Scan(&fb.FundingSourceID, &fb.Cost30d); err != nil {
			return out, err
		}
		out.FundingBurn = append(out.FundingBurn, fb)
	}
	return out, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string, driverUserID *string) (domain.FuelLog, error) {
	if driverUserID != nil && *driverUserID != "" {
		q := fmt.Sprintf(`SELECT%s FROM fuel_logs fl
WHERE fl.id = $1
  AND EXISTS (
      SELECT 1 FROM ambulance_crew_assignments ca
      WHERE ca.ambulance_id = fl.ambulance_id
        AND ca.driver_user_id = $2
        AND ca.active = TRUE
  )`, fuelLogColumns)
		return scanFuelLog(r.db.QueryRow(ctx, q, id, *driverUserID))
	}
	q := fmt.Sprintf(`SELECT%s FROM fuel_logs fl WHERE fl.id = $1`, fuelLogColumns)
	return scanFuelLog(r.db.QueryRow(ctx, q, id))
}

func (r *Repository) Create(ctx context.Context, in domain.FuelLog) (domain.FuelLog, error) {
	const q = `
INSERT INTO fuel_logs (
	id, ambulance_id, fuel_type, liters, unit_cost, cost, odometer_km, station_name,
	filled_at, filled_by, notes, public_token, funding_source_id, created_at, updated_at
)
VALUES (
	gen_random_uuid(), $1,$2,$3,$4,$5,$6,$7,
	$8,$9,$10,$11,$12, now(), now()
)
RETURNING id`

	var id string
	filledAt := in.FilledAt
	if filledAt.IsZero() {
		filledAt = time.Now().UTC()
	}

	if err := r.db.QueryRow(
		ctx,
		q,
		in.AmbulanceID,
		in.FuelType,
		in.Liters,
		in.UnitCost,
		in.Cost,
		in.OdometerKM,
		in.StationName,
		filledAt,
		in.FilledBy,
		in.Notes,
		in.PublicToken,
		in.FundingSourceID,
	).Scan(&id); err != nil {
		return domain.FuelLog{}, err
	}
	return r.GetByID(ctx, id, nil)
}

func (r *Repository) Update(ctx context.Context, id string, req fuelapp.UpdateFuelLogRequest) (domain.FuelLog, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	pos := 1

	if req.FuelType != nil {
		sets = append(sets, fmt.Sprintf("fuel_type = $%d", pos))
		args = append(args, *req.FuelType)
		pos++
	}
	if req.Liters != nil {
		sets = append(sets, fmt.Sprintf("liters = $%d", pos))
		args = append(args, *req.Liters)
		pos++
	}
	if req.UnitCost != nil {
		sets = append(sets, fmt.Sprintf("unit_cost = $%d", pos))
		args = append(args, *req.UnitCost)
		pos++
	}
	if req.Cost != nil {
		sets = append(sets, fmt.Sprintf("cost = $%d", pos))
		args = append(args, *req.Cost)
		pos++
	}
	if req.OdometerKM != nil {
		sets = append(sets, fmt.Sprintf("odometer_km = $%d", pos))
		args = append(args, *req.OdometerKM)
		pos++
	}
	if req.StationName != nil {
		sets = append(sets, fmt.Sprintf("station_name = $%d", pos))
		args = append(args, *req.StationName)
		pos++
	}
	if req.FilledAt != nil {
		sets = append(sets, fmt.Sprintf("filled_at = $%d", pos))
		args = append(args, *req.FilledAt)
		pos++
	}
	if req.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = $%d", pos))
		args = append(args, *req.Notes)
		pos++
	}
	if req.FundingSourceID != nil {
		// Empty string clears the link.
		sets = append(sets, fmt.Sprintf("funding_source_id = NULLIF($%d,'')::uuid", pos))
		args = append(args, *req.FundingSourceID)
		pos++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id, nil)
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	q := fmt.Sprintf("UPDATE fuel_logs SET %s WHERE id = $%d", strings.Join(sets, ", "), pos)
	if _, err := r.db.Exec(ctx, q, args...); err != nil {
		return domain.FuelLog{}, err
	}
	return r.GetByID(ctx, id, nil)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM fuel_logs WHERE id = $1`, id)
	return err
}

// fullName joins a user's first and last name, returning nil when both are empty.
func fullName(first, last *string) *string {
	parts := make([]string, 0, 2)
	if first != nil && strings.TrimSpace(*first) != "" {
		parts = append(parts, strings.TrimSpace(*first))
	}
	if last != nil && strings.TrimSpace(*last) != "" {
		parts = append(parts, strings.TrimSpace(*last))
	}
	if len(parts) == 0 {
		return nil
	}
	name := strings.Join(parts, " ")
	return &name
}

func (r *Repository) GetPublicByToken(ctx context.Context, token string) (domain.FuelLogPublicView, error) {
	const q = `
SELECT
	fl.id, fl.ambulance_id, fl.fuel_type, fl.liters, fl.unit_cost, fl.cost, fl.odometer_km,
	fl.station_name, fl.filled_at, fl.filled_by, fl.notes, fl.public_token,
	fl.dispensed_at, fl.dispense_confirmed, fl.attendant_name, fl.attendant_phone,
	fl.attendant_notes, fl.confirmed_at, fl.created_at, fl.updated_at,
	a.plate_number, a.code, a.make, a.model,
	fb.first_name, fb.last_name,
	dr.first_name, dr.last_name, dr.phone,
	md.first_name, md.last_name, md.phone,
	nu.first_name, nu.last_name, nu.phone,
	dc.first_name, dc.last_name, dc.phone
FROM fuel_logs fl
JOIN ambulances a ON a.id = fl.ambulance_id
LEFT JOIN users fb ON fb.id = fl.filled_by
LEFT JOIN ambulance_crew_assignments ca ON ca.ambulance_id = fl.ambulance_id AND ca.active = TRUE
LEFT JOIN users dr ON dr.id = ca.driver_user_id
LEFT JOIN users md ON md.id = ca.medic_user_id
LEFT JOIN users nu ON nu.id = ca.nurse_user_id
LEFT JOIN users dc ON dc.id = ca.doctor_user_id
WHERE fl.public_token = $1`

	var fl domain.FuelLog
	var fuelType, stationName, filledBy, notes *string
	var attendantName, attendantPhone, attendantNotes *string
	var dispensedAt, confirmedAt *time.Time
	var unitCost, cost *float64
	var odometerKM *int

	var plate string
	var ambCode, ambMake, ambModel *string
	var fbFirst, fbLast *string
	var drFirst, drLast, drPhone *string
	var mdFirst, mdLast, mdPhone *string
	var nuFirst, nuLast, nuPhone *string
	var dcFirst, dcLast, dcPhone *string

	if err := r.db.QueryRow(ctx, q, token).Scan(
		&fl.ID, &fl.AmbulanceID, &fuelType, &fl.Liters, &unitCost, &cost, &odometerKM,
		&stationName, &fl.FilledAt, &filledBy, &notes, &fl.PublicToken,
		&dispensedAt, &fl.DispenseConfirmed, &attendantName, &attendantPhone,
		&attendantNotes, &confirmedAt, &fl.CreatedAt, &fl.UpdatedAt,
		&plate, &ambCode, &ambMake, &ambModel,
		&fbFirst, &fbLast,
		&drFirst, &drLast, &drPhone,
		&mdFirst, &mdLast, &mdPhone,
		&nuFirst, &nuLast, &nuPhone,
		&dcFirst, &dcLast, &dcPhone,
	); err != nil {
		return domain.FuelLogPublicView{}, err
	}

	fl.FuelType = fuelType
	fl.UnitCost = unitCost
	fl.Cost = cost
	fl.OdometerKM = odometerKM
	fl.StationName = stationName
	fl.FilledBy = filledBy
	fl.Notes = notes
	fl.DispensedAt = dispensedAt
	fl.AttendantName = attendantName
	fl.AttendantPhone = attendantPhone
	fl.AttendantNotes = attendantNotes
	fl.ConfirmedAt = confirmedAt

	view := domain.FuelLogPublicView{
		FuelLog:        fl,
		AmbulancePlate: plate,
		AmbulanceCode:  ambCode,
		AmbulanceMake:  ambMake,
		AmbulanceModel: ambModel,
		LoggedByName:   fullName(fbFirst, fbLast),
		Crew:           make([]domain.CrewMember, 0, 4),
	}

	addCrew := func(role string, first, last, phone *string) {
		name := fullName(first, last)
		if name == nil {
			return
		}
		view.Crew = append(view.Crew, domain.CrewMember{Role: role, Name: *name, Phone: phone})
	}
	addCrew("Driver", drFirst, drLast, drPhone)
	addCrew("Medic", mdFirst, mdLast, mdPhone)
	addCrew("Nurse", nuFirst, nuLast, nuPhone)
	addCrew("Doctor", dcFirst, dcLast, dcPhone)

	return view, nil
}

func (r *Repository) ConfirmDispense(ctx context.Context, token string, req fuelapp.ConfirmFuelDispenseRequest) (int64, error) {
	const q = `
UPDATE fuel_logs
SET attendant_name     = $2,
    attendant_phone    = $3,
    attendant_notes    = $4,
    dispensed_at       = COALESCE($5, now()),
    dispense_confirmed = $6,
    confirmed_at       = CASE WHEN $6 THEN now() ELSE NULL END,
    odometer_km        = COALESCE($7, odometer_km),
    updated_at         = now()
WHERE public_token = $1 AND dispense_confirmed = FALSE`

	tag, err := r.db.Exec(ctx, q, token, req.AttendantName, req.AttendantPhone, req.Notes, req.DispensedAt, req.Approved, req.OdometerKM)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// ── Funding sources ─────────────────────────────────────────────────────────

// fundingSourceSelect is the shared projection for a funding source with its
// derived spent / top-up / remaining figures.
const fundingSourceSelect = `
	SELECT s.id, s.organisation_name, s.funding_date, s.amount, s.notes,
		COALESCE((SELECT SUM(fl.cost) FROM fuel_logs fl WHERE fl.funding_source_id = s.id), 0) AS spent,
		COALESCE((SELECT SUM(t.amount) FROM fuel_funding_topups t WHERE t.funding_source_id = s.id), 0) AS topup_total,
		COALESCE((SELECT COUNT(*) FROM fuel_funding_topups t WHERE t.funding_source_id = s.id), 0) AS topup_count,
		s.created_at, s.updated_at
	FROM fuel_funding_sources s`

func scanFundingSource(row interface {
	Scan(dest ...any) error
}) (domain.FundingSource, error) {
	var fs domain.FundingSource
	if err := row.Scan(
		&fs.ID, &fs.OrganisationName, &fs.FundingDate, &fs.Amount, &fs.Notes,
		&fs.Spent, &fs.TopupTotal, &fs.TopupCount, &fs.CreatedAt, &fs.UpdatedAt,
	); err != nil {
		return domain.FundingSource{}, err
	}
	fs.TotalFunded = fs.Amount + fs.TopupTotal
	fs.Remaining = fs.TotalFunded - fs.Spent
	return fs, nil
}

func (r *Repository) ListFundingSources(ctx context.Context) ([]domain.FundingSource, error) {
	rows, err := r.db.Query(ctx, fundingSourceSelect+`
		ORDER BY s.funding_date DESC, s.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.FundingSource, 0)
	for rows.Next() {
		fs, err := scanFundingSource(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, fs)
	}
	return items, rows.Err()
}

func (r *Repository) GetFundingSource(ctx context.Context, id string) (domain.FundingSource, error) {
	return scanFundingSource(r.db.QueryRow(ctx, fundingSourceSelect+` WHERE s.id = $1`, id))
}

func (r *Repository) CreateFundingSource(ctx context.Context, in domain.FundingSource) (domain.FundingSource, error) {
	var id string
	if err := r.db.QueryRow(ctx, `
		INSERT INTO fuel_funding_sources (organisation_name, funding_date, amount, notes)
		VALUES ($1, COALESCE($2, CURRENT_DATE), $3, $4)
		RETURNING id
	`, in.OrganisationName, nullableTime(in.FundingDate), in.Amount, in.Notes).Scan(&id); err != nil {
		return domain.FundingSource{}, err
	}
	return r.GetFundingSource(ctx, id)
}

func (r *Repository) UpdateFundingSource(ctx context.Context, id string, req fuelapp.UpdateFundingSourceRequest) (domain.FundingSource, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	pos := 1

	if req.OrganisationName != nil {
		sets = append(sets, fmt.Sprintf("organisation_name = $%d", pos))
		args = append(args, *req.OrganisationName)
		pos++
	}
	if req.FundingDate != nil {
		d, err := time.Parse("2006-01-02", *req.FundingDate)
		if err != nil {
			return domain.FundingSource{}, err
		}
		sets = append(sets, fmt.Sprintf("funding_date = $%d", pos))
		args = append(args, d)
		pos++
	}
	if req.Amount != nil {
		sets = append(sets, fmt.Sprintf("amount = $%d", pos))
		args = append(args, *req.Amount)
		pos++
	}
	if req.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = NULLIF($%d, '')", pos))
		args = append(args, *req.Notes)
		pos++
	}

	if len(sets) == 0 {
		return r.GetFundingSource(ctx, id)
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	if _, err := r.db.Exec(ctx, fmt.Sprintf(
		`UPDATE fuel_funding_sources SET %s WHERE id = $%d`, strings.Join(sets, ", "), pos), args...); err != nil {
		return domain.FundingSource{}, err
	}
	return r.GetFundingSource(ctx, id)
}

func (r *Repository) DeleteFundingSource(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM fuel_funding_sources WHERE id = $1`, id)
	return err
}

func (r *Repository) AddFundingTopup(ctx context.Context, in domain.FundingTopup) (domain.FundingTopup, error) {
	err := r.db.QueryRow(ctx, `
		INSERT INTO fuel_funding_topups (funding_source_id, amount, topup_date, notes, created_by)
		VALUES ($1, $2, COALESCE($3, CURRENT_DATE), NULLIF($4, ''), $5)
		RETURNING id, funding_source_id, amount, topup_date, notes, created_by, created_at
	`, in.FundingSourceID, in.Amount, nullableTime(in.TopupDate), nullableStr(in.Notes), in.CreatedBy).Scan(
		&in.ID, &in.FundingSourceID, &in.Amount, &in.TopupDate, &in.Notes, &in.CreatedBy, &in.CreatedAt,
	)
	if err != nil {
		return domain.FundingTopup{}, err
	}
	return in, nil
}

func (r *Repository) ListFundingTopups(ctx context.Context, fundingSourceID string) ([]domain.FundingTopup, error) {
	rows, err := r.db.Query(ctx, `
		SELECT t.id, t.funding_source_id, t.amount, t.topup_date, t.notes, t.created_by,
			(SELECT (u.first_name || ' ' || u.last_name) FROM users u WHERE u.id = t.created_by) AS created_by_name,
			t.created_at
		FROM fuel_funding_topups t
		WHERE t.funding_source_id = $1
		ORDER BY t.topup_date DESC, t.created_at DESC
	`, fundingSourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]domain.FundingTopup, 0)
	for rows.Next() {
		var t domain.FundingTopup
		if err := rows.Scan(
			&t.ID, &t.FundingSourceID, &t.Amount, &t.TopupDate, &t.Notes, &t.CreatedBy, &t.CreatedByName, &t.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func nullableStr(s *string) any {
	if s == nil {
		return ""
	}
	return *s
}

// nullableTime maps the zero time to NULL so SQL defaults apply.
func nullableTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
