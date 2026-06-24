package infrastructure

import (
	"context"
	"fmt"
	"strings"

	respapp "dispatch/internal/modules/responders/application"
	"dispatch/internal/modules/responders/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ respapp.Repository = (*Repository)(nil)

// activeDispatchStatuses are the dispatch_assignments states that count as the
// ambulance currently being engaged on a case.
const activeDispatchStatuses = `'PROPOSED','ASSIGNED','ACCEPTED','DEPARTED','ARRIVED_SCENE','PATIENT_LOADED','ARRIVED_DESTINATION'`

// responderColumns is the shared projection used by both list and get. Column
// order must match scanResponder.
var responderColumns = fmt.Sprintf(`
	a.id,
	COALESCE(NULLIF(a.code, ''), a.plate_number, ''),
	a.status,
	a.dispatch_readiness,
	a.is_active,
	COALESCE(d.name, ''),
	COALESCE(f.name, ''),
	COALESCE(du.first_name, ''),
	COALESCE(du.last_name, ''),
	COALESCE(du.phone, ''),
	COALESCE(cat.name, ''),
	COALESCE(cat.supports_maternal, FALSE),
	COALESCE(cat.supports_neonatal, FALSE),
	COALESCE(cat.supports_trauma, FALSE),
	COALESCE(cat.supports_critical_care, FALSE),
	COALESCE(cat.supports_referral, FALSE),
	COALESCE(ac.cnt, 0)`)

const responderJoins = `
	FROM ambulances a
	LEFT JOIN ambulance_crew_assignments ca ON ca.ambulance_id = a.id AND ca.active = TRUE
	LEFT JOIN users du ON du.id = ca.driver_user_id
	LEFT JOIN ref_districts d ON d.id = a.district_id
	LEFT JOIN ref_facilities f ON f.id = a.station_facility_id
	LEFT JOIN ref_ambulance_categories cat ON cat.id = a.category_id
	LEFT JOIN LATERAL (
		SELECT COUNT(1) AS cnt
		FROM dispatch_assignments da
		WHERE da.ambulance_id = a.id
		  AND da.status IN (` + activeDispatchStatuses + `)
	) ac ON TRUE`

type responderRow struct {
	id                                                          string
	unit, status, readiness                                     string
	isActive                                                    bool
	district, station, firstName, lastName, phone, categoryName string
	maternal, neonatal, trauma, criticalCare, referral          bool
	activeCount                                                 int
}

func scanResponder(scan func(dest ...any) error) (domain.Responder, error) {
	var r responderRow
	if err := scan(
		&r.id,
		&r.unit,
		&r.status,
		&r.readiness,
		&r.isActive,
		&r.district,
		&r.station,
		&r.firstName,
		&r.lastName,
		&r.phone,
		&r.categoryName,
		&r.maternal,
		&r.neonatal,
		&r.trauma,
		&r.criticalCare,
		&r.referral,
		&r.activeCount,
	); err != nil {
		return domain.Responder{}, err
	}
	return mapResponder(r), nil
}

func mapResponder(r responderRow) domain.Responder {
	name := strings.TrimSpace(r.firstName + " " + r.lastName)
	if name == "" {
		name = "Unassigned crew"
	}

	base := r.station
	if base == "" {
		base = r.district
	}

	return domain.Responder{
		ID:                     r.id,
		Name:                   name,
		AmbulanceUnit:          r.unit,
		CrewType:               crewType(r),
		VehicleType:            "Road Ambulance",
		District:               r.district,
		Base:                   base,
		Status:                 responderStatus(r),
		ETAMinutes:             0,
		CurrentAssignmentCount: r.activeCount,
		Capabilities:           capabilities(r),
		Phone:                  r.phone,
		NearestLandmark:        base,
	}
}

// crewType maps the ambulance category's clinical support flags to the crew
// type vocabulary the dispatch console understands.
func crewType(r responderRow) string {
	switch {
	case r.criticalCare:
		return "ALS"
	case r.maternal || r.neonatal:
		return "Maternity"
	default:
		return "BLS"
	}
}

// responderStatus derives Available/Busy/Offline from the ambulance status,
// dispatch readiness and current active assignment count.
func responderStatus(r responderRow) string {
	s := strings.ToUpper(strings.TrimSpace(r.status))
	if !r.isActive ||
		strings.ToUpper(strings.TrimSpace(r.readiness)) == "NOT_DISPATCHABLE" ||
		s == "OFFLINE" || s == "RETIRED" || s == "MAINTENANCE" || s == "BREAKDOWN" {
		return "Offline"
	}
	if r.activeCount > 0 ||
		s == "RESERVED" || s == "ASSIGNED" || s == "ENROUTE" || s == "AT_SCENE" ||
		s == "TRANSPORTING" || s == "RETURNING" {
		return "Busy"
	}
	return "Available"
}

func capabilities(r responderRow) []string {
	caps := make([]string, 0, 5)
	if r.maternal {
		caps = append(caps, "Maternal")
	}
	if r.neonatal {
		caps = append(caps, "Neonatal")
	}
	if r.trauma {
		caps = append(caps, "Trauma")
	}
	if r.criticalCare {
		caps = append(caps, "Critical Care")
	}
	if r.referral {
		caps = append(caps, "Referral")
	}
	if len(caps) == 0 {
		if r.categoryName != "" {
			caps = append(caps, r.categoryName)
		} else {
			caps = append(caps, crewType(r))
		}
	}
	return caps
}

func (r *Repository) ListResponders(ctx context.Context, p platformdb.Pagination) ([]domain.Responder, int64, error) {
	allowedSorts := map[string]string{
		"created_at":   "a.created_at",
		"plate_number": "a.plate_number",
		"status":       "a.status",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			COALESCE(a.code,'') ILIKE $%d OR
			a.plate_number ILIKE $%d OR
			COALESCE(du.first_name,'') ILIKE $%d OR
			COALESCE(du.last_name,'') ILIKE $%d OR
			COALESCE(d.name,'') ILIKE $%d
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
		case "district_id":
			where = append(where, fmt.Sprintf("a.district_id = $%d", argPos))
			args = append(args, value)
			argPos++
		case "category_id":
			where = append(where, fmt.Sprintf("a.category_id = $%d", argPos))
			args = append(args, value)
			argPos++
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	countSQL := fmt.Sprintf(`SELECT COUNT(1) %s %s`, responderJoins, whereSQL)
	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)
	listSQL := fmt.Sprintf(`SELECT %s %s %s %s LIMIT $%d OFFSET $%d`,
		responderColumns, responderJoins, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, listSQL, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.Responder, 0)
	for rows.Next() {
		item, err := scanResponder(rows.Scan)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string) (domain.Responder, error) {
	query := fmt.Sprintf(`SELECT %s %s WHERE a.id = $1`, responderColumns, responderJoins)
	return scanResponder(r.db.QueryRow(ctx, query, id).Scan)
}
