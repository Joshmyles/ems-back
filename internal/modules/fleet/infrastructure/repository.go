package infrastructure

import (
	"context"
	"fmt"
	"strings"

	fleetapp "dispatch/internal/modules/fleet/application"
	"dispatch/internal/modules/fleet/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ fleetapp.Repository = (*Repository)(nil)

func (r *Repository) ListAmbulances(ctx context.Context, p platformdb.Pagination, driverUserID *string) ([]domain.Ambulance, int64, error) {
	allowedSorts := map[string]string{
		"created_at":         "a.created_at",
		"plate_number":       "a.plate_number",
		"status":             "a.status",
		"dispatch_readiness": "a.dispatch_readiness",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if driverUserID != nil && *driverUserID != "" {
		where = append(where, fmt.Sprintf(`(
			EXISTS (
				SELECT 1 FROM ambulance_crew_assignments ca2
				WHERE ca2.ambulance_id = a.id
				  AND ca2.driver_user_id = $%d
				  AND ca2.active = TRUE
			)
			OR EXISTS (
				SELECT 1 FROM dispatch_assignments da
				WHERE da.ambulance_id = a.id
				  AND da.driver_user_id = $%d
				  AND da.status IN ('PROPOSED','ASSIGNED','ACCEPTED','DEPARTED','ARRIVED_SCENE','PATIENT_LOADED','ARRIVED_DESTINATION')
			)
		)`, argPos, argPos))
		args = append(args, *driverUserID)
		argPos++
	}

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			COALESCE(a.code,'') ILIKE $%d OR
			a.plate_number ILIKE $%d OR
			COALESCE(a.vin,'') ILIKE $%d OR
			COALESCE(a.make,'') ILIKE $%d OR
			COALESCE(a.model,'') ILIKE $%d
		)`, argPos, argPos, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	for key, value := range p.Filters {
		switch key {
		case "status":
			where = append(where, fmt.Sprintf("a.status = $%d", argPos))
			args = append(args, strings.ToUpper(value))
			argPos++
		case "dispatch_readiness":
			where = append(where, fmt.Sprintf("a.dispatch_readiness = $%d", argPos))
			args = append(args, strings.ToUpper(value))
			argPos++
		case "district_id":
			where = append(where, fmt.Sprintf("a.district_id = $%d", argPos))
			args = append(args, value)
			argPos++
		case "category_id":
			where = append(where, fmt.Sprintf("a.category_id = $%d", argPos))
			args = append(args, value)
			argPos++
		case "date_from":
			where = append(where, fmt.Sprintf("a.created_at >= $%d", argPos))
			args = append(args, value)
			argPos++
		case "date_to":
			where = append(where, fmt.Sprintf("a.created_at <= $%d", argPos))
			args = append(args, value)
			argPos++
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	countSQL := fmt.Sprintf(`SELECT COUNT(1) FROM ambulances a %s`, whereSQL)

	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)

	listSQL := fmt.Sprintf(`
		SELECT
			a.id,
			COALESCE(a.code, ''),
			a.plate_number,
			COALESCE(a.vin, ''),
			COALESCE(a.make, ''),
			COALESCE(a.model, ''),
			a.year_of_manufacture,
			a.category_id,
			COALESCE(a.ownership_type, ''),
			a.station_facility_id,
			a.district_id,
			a.status,
			a.dispatch_readiness,
			a.gps_lat,
			a.gps_lon,
			a.last_seen_at,
			a.is_active,
			a.created_at,
			a.updated_at,
			ca.driver_user_id,
			du.first_name,
			du.last_name,
			du.phone
		FROM ambulances a
		LEFT JOIN ambulance_crew_assignments ca
			ON ca.ambulance_id = a.id AND ca.active = TRUE
		LEFT JOIN users du
			ON du.id = ca.driver_user_id
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, listSQL, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.Ambulance, 0)
	for rows.Next() {
		var a domain.Ambulance
		var code, vin, makeVal, model, ownershipType *string
		var driverID, driverFirst, driverLast, driverPhone *string
		if err := rows.Scan(
			&a.ID,
			&code,
			&a.PlateNumber,
			&vin,
			&makeVal,
			&model,
			&a.YearOfManufacture,
			&a.CategoryID,
			&ownershipType,
			&a.StationFacilityID,
			&a.DistrictID,
			&a.Status,
			&a.DispatchReadiness,
			&a.GPSLat,
			&a.GPSLon,
			&a.LastSeenAt,
			&a.IsActive,
			&a.CreatedAt,
			&a.UpdatedAt,
			&driverID,
			&driverFirst,
			&driverLast,
			&driverPhone,
		); err != nil {
			return nil, 0, err
		}
		a.Code = code
		a.VIN = vin
		a.Make = makeVal
		a.Model = model
		a.OwnershipType = ownershipType
		a.CurrentDriverUserID = driverID
		a.CurrentDriverFirstName = driverFirst
		a.CurrentDriverLastName = driverLast
		a.CurrentDriverPhone = driverPhone
		items = append(items, a)
	}

	return items, total, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string, driverUserID *string) (domain.Ambulance, error) {
	baseQuery := `
SELECT
	a.id,
	a.code,
	a.plate_number,
	a.vin,
	a.make,
	a.model,
	a.year_of_manufacture,
	a.category_id,
	a.ownership_type,
	a.station_facility_id,
	a.district_id,
	a.status,
	a.dispatch_readiness,
	a.gps_lat,
	a.gps_lon,
	a.last_seen_at,
	a.is_active,
	a.created_at,
	a.updated_at,
	ca.driver_user_id,
	du.first_name,
	du.last_name,
	du.phone
FROM ambulances a
LEFT JOIN ambulance_crew_assignments ca
	ON ca.ambulance_id = a.id AND ca.active = TRUE
LEFT JOIN users du
	ON du.id = ca.driver_user_id
WHERE a.id = $1`

	args := []any{id}
	q := baseQuery
	if driverUserID != nil && *driverUserID != "" {
		q = baseQuery + ` AND EXISTS (
			SELECT 1
			FROM (
				SELECT ca2.ambulance_id
				FROM ambulance_crew_assignments ca2
				WHERE ca2.ambulance_id = a.id
				  AND ca2.driver_user_id = $2
				  AND ca2.active = TRUE
				UNION
				SELECT da.ambulance_id
				FROM dispatch_assignments da
				WHERE da.ambulance_id = a.id
				  AND da.driver_user_id = $2
				  AND da.status IN ('PROPOSED','ASSIGNED','ACCEPTED','DEPARTED','ARRIVED_SCENE','PATIENT_LOADED','ARRIVED_DESTINATION')
			) scoped_ambulance
		)`
		args = append(args, *driverUserID)
	}

	var a domain.Ambulance
	if err := r.db.QueryRow(ctx, q, args...).Scan(
		&a.ID,
		&a.Code,
		&a.PlateNumber,
		&a.VIN,
		&a.Make,
		&a.Model,
		&a.YearOfManufacture,
		&a.CategoryID,
		&a.OwnershipType,
		&a.StationFacilityID,
		&a.DistrictID,
		&a.Status,
		&a.DispatchReadiness,
		&a.GPSLat,
		&a.GPSLon,
		&a.LastSeenAt,
		&a.IsActive,
		&a.CreatedAt,
		&a.UpdatedAt,
		&a.CurrentDriverUserID,
		&a.CurrentDriverFirstName,
		&a.CurrentDriverLastName,
		&a.CurrentDriverPhone,
	); err != nil {
		return domain.Ambulance{}, err
	}
	return a, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Ambulance) (domain.Ambulance, error) {
	const q = `
INSERT INTO ambulances (
	id, code, plate_number, vin, make, model, year_of_manufacture,
	category_id, ownership_type, station_facility_id, district_id,
	status, dispatch_readiness, gps_lat, gps_lon, location, last_seen_at,
	is_active, created_at, updated_at
)
VALUES (
	gen_random_uuid(), $1,$2,$3,$4,$5,$6,
	$7,$8,$9,$10,
	$11,$12,NULL,NULL,NULL,NULL,
	TRUE, now(), now()
)
RETURNING id`
	var id string
	if err := r.db.QueryRow(
		ctx,
		q,
		in.Code,
		in.PlateNumber,
		in.VIN,
		in.Make,
		in.Model,
		in.YearOfManufacture,
		in.CategoryID,
		in.OwnershipType,
		in.StationFacilityID,
		in.DistrictID,
		in.Status,
		in.DispatchReadiness,
	).Scan(&id); err != nil {
		return domain.Ambulance{}, err
	}
	return r.GetByID(ctx, id, nil)
}

func (r *Repository) Update(ctx context.Context, id string, req fleetapp.UpdateAmbulanceRequest) (domain.Ambulance, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	pos := 1

	if req.Code != nil {
		sets = append(sets, fmt.Sprintf("code = $%d", pos))
		args = append(args, *req.Code)
		pos++
	}
	if req.VIN != nil {
		sets = append(sets, fmt.Sprintf("vin = $%d", pos))
		args = append(args, *req.VIN)
		pos++
	}
	if req.Make != nil {
		sets = append(sets, fmt.Sprintf("make = $%d", pos))
		args = append(args, *req.Make)
		pos++
	}
	if req.Model != nil {
		sets = append(sets, fmt.Sprintf("model = $%d", pos))
		args = append(args, *req.Model)
		pos++
	}
	if req.YearOfManufacture != nil {
		sets = append(sets, fmt.Sprintf("year_of_manufacture = $%d", pos))
		args = append(args, *req.YearOfManufacture)
		pos++
	}
	if req.CategoryID != nil {
		sets = append(sets, fmt.Sprintf("category_id = $%d", pos))
		args = append(args, *req.CategoryID)
		pos++
	}
	if req.OwnershipType != nil {
		sets = append(sets, fmt.Sprintf("ownership_type = $%d", pos))
		args = append(args, *req.OwnershipType)
		pos++
	}
	if req.StationFacilityID != nil {
		sets = append(sets, fmt.Sprintf("station_facility_id = $%d", pos))
		args = append(args, *req.StationFacilityID)
		pos++
	}
	if req.DistrictID != nil {
		sets = append(sets, fmt.Sprintf("district_id = $%d", pos))
		args = append(args, *req.DistrictID)
		pos++
	}
	if req.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", pos))
		args = append(args, strings.ToUpper(*req.Status))
		pos++
	}
	if req.DispatchReadiness != nil {
		sets = append(sets, fmt.Sprintf("dispatch_readiness = $%d", pos))
		args = append(args, strings.ToUpper(*req.DispatchReadiness))
		pos++
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id, nil)
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE ambulances SET %s WHERE id = $%d", strings.Join(sets, ", "), pos)
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return domain.Ambulance{}, err
	}
	return r.GetByID(ctx, id, nil)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM ambulances WHERE id = $1`, id)
	return err
}

// AssignDriver sets driver_user_id on the active crew assignment for the
// ambulance. If no active row exists one is created; otherwise the existing
// active row is updated in place so other crew slots (medic/nurse/doctor) are
// preserved.
func (r *Repository) AssignDriver(ctx context.Context, ambulanceID, driverUserID string) error {
	tag, err := r.db.Exec(ctx, `
		UPDATE ambulance_crew_assignments
		SET driver_user_id = $2
		WHERE ambulance_id = $1 AND active = TRUE
	`, ambulanceID, driverUserID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() > 0 {
		return nil
	}
	_, err = r.db.Exec(ctx, `
		INSERT INTO ambulance_crew_assignments (ambulance_id, driver_user_id, active)
		VALUES ($1, $2, TRUE)
	`, ambulanceID, driverUserID)
	return err
}

// UnassignDriver clears the driver_user_id from the active crew assignment.
// The row is left active so other crew members (if any) are not affected.
func (r *Repository) UnassignDriver(ctx context.Context, ambulanceID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE ambulance_crew_assignments
		SET driver_user_id = NULL
		WHERE ambulance_id = $1 AND active = TRUE
	`, ambulanceID)
	return err
}
