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
	fl.updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

// scanFuelLog reads a fuel log row projected via fuelLogColumns.
func scanFuelLog(row rowScanner) (domain.FuelLog, error) {
	var fl domain.FuelLog
	var fuelType, stationName, filledBy, notes *string
	var attendantName, attendantPhone, attendantNotes *string
	var dispensedAt, confirmedAt *time.Time
	var cost *float64
	var odometerKM *int

	if err := row.Scan(
		&fl.ID,
		&fl.AmbulanceID,
		&fuelType,
		&fl.Liters,
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
	); err != nil {
		return domain.FuelLog{}, err
	}

	fl.FuelType = fuelType
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

	if driverUserID != nil && *driverUserID != "" {
		where = append(where, fmt.Sprintf(`EXISTS (
			SELECT 1 FROM ambulance_crew_assignments ca
			WHERE ca.ambulance_id = fl.ambulance_id
			  AND ca.driver_user_id = $%d
			  AND ca.active = TRUE
		)`, pos))
		args = append(args, *driverUserID)
		pos++
	}

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			COALESCE(fl.fuel_type,'') ILIKE $%d OR
			COALESCE(fl.station_name,'') ILIKE $%d OR
			COALESCE(fl.notes,'') ILIKE $%d
		)`, pos, pos, pos))
		args = append(args, "%"+p.Search+"%")
		pos++
	}

	for k, v := range p.Filters {
		switch k {
		case "ambulance_id":
			where = append(where, fmt.Sprintf("fl.ambulance_id = $%d", pos))
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
	id, ambulance_id, fuel_type, liters, cost, odometer_km, station_name,
	filled_at, filled_by, notes, public_token, created_at, updated_at
)
VALUES (
	gen_random_uuid(), $1,$2,$3,$4,$5,$6,
	$7,$8,$9,$10, now(), now()
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
		in.Cost,
		in.OdometerKM,
		in.StationName,
		filledAt,
		in.FilledBy,
		in.Notes,
		in.PublicToken,
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
	fl.id, fl.ambulance_id, fl.fuel_type, fl.liters, fl.cost, fl.odometer_km,
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
	var cost *float64
	var odometerKM *int

	var plate string
	var ambCode, ambMake, ambModel *string
	var fbFirst, fbLast *string
	var drFirst, drLast, drPhone *string
	var mdFirst, mdLast, mdPhone *string
	var nuFirst, nuLast, nuPhone *string
	var dcFirst, dcLast, dcPhone *string

	if err := r.db.QueryRow(ctx, q, token).Scan(
		&fl.ID, &fl.AmbulanceID, &fuelType, &fl.Liters, &cost, &odometerKM,
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
    updated_at         = now()
WHERE public_token = $1 AND dispense_confirmed = FALSE`

	tag, err := r.db.Exec(ctx, q, token, req.AttendantName, req.AttendantPhone, req.Notes, req.DispensedAt, req.Approved)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
