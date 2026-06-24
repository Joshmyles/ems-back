package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"dispatch/internal/modules/trips/application"
	"dispatch/internal/modules/trips/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ application.Repository = (*Repository)(nil)

func (r *Repository) ListTrips(ctx context.Context, p platformdb.Pagination) ([]domain.Trip, int64, error) {
	allowedSorts := map[string]string{
		"started_at": "t.started_at",
		"created_at": "t.created_at",
	}
	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	for key, value := range p.Filters {
		switch key {
		case "incident_id":
			where = append(where, fmt.Sprintf("t.incident_id = $%d", argPos))
			args = append(args, value)
			argPos++
		case "ambulance_id":
			where = append(where, fmt.Sprintf("t.ambulance_id = $%d", argPos))
			args = append(args, value)
			argPos++
		case "date_from":
			where = append(where, fmt.Sprintf("t.started_at >= $%d", argPos))
			args = append(args, value)
			argPos++
		case "date_to":
			where = append(where, fmt.Sprintf("t.started_at <= $%d", argPos))
			args = append(args, value)
			argPos++
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	countSQL := fmt.Sprintf(`SELECT COUNT(1) FROM trips t %s`, whereSQL)
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)

	listSQL := fmt.Sprintf(`
SELECT
	t.id,
	t.dispatch_assignment_id,
	t.incident_id,
	t.ambulance_id,
	t.origin_lat,
	t.origin_lon,
	t.scene_lat,
	t.scene_lon,
	t.destination_facility_id,
	t.destination_lat,
	t.destination_lon,
	t.odometer_start,
	t.odometer_end,
	t.started_at,
	t.ended_at,
	t.outcome,
	t.notes,
	t.created_at,
	t.updated_at,
	COALESCE(i.incident_number,''),
	COALESCE(a.code,''),
	COALESCE(a.plate_number,''),
	COALESCE(df.name,'')
FROM trips t
LEFT JOIN incidents i ON i.id = t.incident_id
LEFT JOIN ambulances a ON a.id = t.ambulance_id
LEFT JOIN ref_facilities df ON df.id = t.destination_facility_id
%s
%s
LIMIT $%d OFFSET $%d`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, listSQL, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.Trip, 0)
	for rows.Next() {
		var t domain.Trip
		if err := rows.Scan(
			&t.ID,
			&t.DispatchAssignmentID,
			&t.IncidentID,
			&t.AmbulanceID,
			&t.OriginLat,
			&t.OriginLon,
			&t.SceneLat,
			&t.SceneLon,
			&t.DestinationFacilityID,
			&t.DestinationLat,
			&t.DestinationLon,
			&t.OdometerStart,
			&t.OdometerEnd,
			&t.StartedAt,
			&t.EndedAt,
			&t.Outcome,
			&t.Notes,
			&t.CreatedAt,
			&t.UpdatedAt,
			&t.IncidentNumber,
			&t.AmbulanceCode,
			&t.AmbulancePlate,
			&t.DestinationFacilityName,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, t)
	}
	return items, total, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string) (domain.Trip, error) {
	const q = `
SELECT
	t.id,
	t.dispatch_assignment_id,
	t.incident_id,
	t.ambulance_id,
	t.origin_lat,
	t.origin_lon,
	t.scene_lat,
	t.scene_lon,
	t.destination_facility_id,
	t.destination_lat,
	t.destination_lon,
	t.odometer_start,
	t.odometer_end,
	t.started_at,
	t.ended_at,
	t.outcome,
	t.notes,
	t.created_at,
	t.updated_at,
	COALESCE(i.incident_number,''),
	COALESCE(a.code,''),
	COALESCE(a.plate_number,''),
	COALESCE(df.name,'')
FROM trips t
LEFT JOIN incidents i ON i.id = t.incident_id
LEFT JOIN ambulances a ON a.id = t.ambulance_id
LEFT JOIN ref_facilities df ON df.id = t.destination_facility_id
WHERE t.id = $1`
	var t domain.Trip
	if err := r.db.QueryRow(ctx, q, id).Scan(
		&t.ID,
		&t.DispatchAssignmentID,
		&t.IncidentID,
		&t.AmbulanceID,
		&t.OriginLat,
		&t.OriginLon,
		&t.SceneLat,
		&t.SceneLon,
		&t.DestinationFacilityID,
		&t.DestinationLat,
		&t.DestinationLon,
		&t.OdometerStart,
		&t.OdometerEnd,
		&t.StartedAt,
		&t.EndedAt,
		&t.Outcome,
		&t.Notes,
		&t.CreatedAt,
		&t.UpdatedAt,
		&t.IncidentNumber,
		&t.AmbulanceCode,
		&t.AmbulancePlate,
		&t.DestinationFacilityName,
	); err != nil {
		return domain.Trip{}, err
	}
	return t, nil
}

func (r *Repository) CreateTrip(ctx context.Context, in domain.Trip) (domain.Trip, error) {
	const q = `
INSERT INTO trips (
	id, dispatch_assignment_id, incident_id, ambulance_id,
	origin_lat, origin_lon, scene_lat, scene_lon,
	destination_facility_id, destination_lat, destination_lon,
	odometer_start, odometer_end, started_at, ended_at, outcome, notes,
	created_at, updated_at
) VALUES (
	$1,$2,$3,$4,
	$5,$6,$7,$8,
	$9,$10,$11,
	$12,$13,$14,$15,$16,$17,
	$18,$19
)
RETURNING created_at, updated_at`
	if err := r.db.QueryRow(
		ctx,
		q,
		in.ID,
		in.DispatchAssignmentID,
		in.IncidentID,
		in.AmbulanceID,
		in.OriginLat,
		in.OriginLon,
		in.SceneLat,
		in.SceneLon,
		in.DestinationFacilityID,
		in.DestinationLat,
		in.DestinationLon,
		in.OdometerStart,
		in.OdometerEnd,
		in.StartedAt,
		in.EndedAt,
		in.Outcome,
		in.Notes,
		in.CreatedAt,
		in.UpdatedAt,
	).Scan(&in.CreatedAt, &in.UpdatedAt); err != nil {
		return domain.Trip{}, err
	}
	return in, nil
}

func (r *Repository) UpdateTrip(ctx context.Context, id string, req application.UpdateTripRequest) (domain.Trip, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	pos := 1

	if req.AmbulanceID != nil {
		sets = append(sets, fmt.Sprintf("ambulance_id = $%d", pos))
		args = append(args, *req.AmbulanceID)
		pos++
	}
	if req.OriginLat != nil {
		sets = append(sets, fmt.Sprintf("origin_lat = $%d", pos))
		args = append(args, *req.OriginLat)
		pos++
	}
	if req.OriginLon != nil {
		sets = append(sets, fmt.Sprintf("origin_lon = $%d", pos))
		args = append(args, *req.OriginLon)
		pos++
	}
	if req.SceneLat != nil {
		sets = append(sets, fmt.Sprintf("scene_lat = $%d", pos))
		args = append(args, *req.SceneLat)
		pos++
	}
	if req.SceneLon != nil {
		sets = append(sets, fmt.Sprintf("scene_lon = $%d", pos))
		args = append(args, *req.SceneLon)
		pos++
	}
	if req.DestinationFacilityID != nil {
		sets = append(sets, fmt.Sprintf("destination_facility_id = $%d", pos))
		args = append(args, *req.DestinationFacilityID)
		pos++
	}
	if req.DestinationLat != nil {
		sets = append(sets, fmt.Sprintf("destination_lat = $%d", pos))
		args = append(args, *req.DestinationLat)
		pos++
	}
	if req.DestinationLon != nil {
		sets = append(sets, fmt.Sprintf("destination_lon = $%d", pos))
		args = append(args, *req.DestinationLon)
		pos++
	}
	if req.OdometerStart != nil {
		sets = append(sets, fmt.Sprintf("odometer_start = $%d", pos))
		args = append(args, *req.OdometerStart)
		pos++
	}
	if req.OdometerEnd != nil {
		sets = append(sets, fmt.Sprintf("odometer_end = $%d", pos))
		args = append(args, *req.OdometerEnd)
		pos++
	}
	if req.StartedAt != nil {
		sets = append(sets, fmt.Sprintf("started_at = $%d", pos))
		args = append(args, *req.StartedAt)
		pos++
	}
	if req.EndedAt != nil {
		sets = append(sets, fmt.Sprintf("ended_at = $%d", pos))
		args = append(args, *req.EndedAt)
		pos++
	}
	if req.Outcome != nil {
		sets = append(sets, fmt.Sprintf("outcome = $%d", pos))
		args = append(args, *req.Outcome)
		pos++
	}
	if req.Notes != nil {
		sets = append(sets, fmt.Sprintf("notes = $%d", pos))
		args = append(args, *req.Notes)
		pos++
	}
	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}
	sets = append(sets, "updated_at = now()")
	args = append(args, id)
	query := fmt.Sprintf("UPDATE trips SET %s WHERE id = $%d", strings.Join(sets, ", "), pos)
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return domain.Trip{}, err
	}
	return r.GetByID(ctx, id)
}

func (r *Repository) DeleteTrip(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM trips WHERE id = $1`, id)
	return err
}

func (r *Repository) ListTripEvents(ctx context.Context, tripID string, p platformdb.Pagination) ([]domain.TripEvent, int64, error) {
	where := "WHERE te.trip_id = $1"
	args := []any{tripID}

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM trip_events te `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
SELECT
	te.id,
	te.trip_id,
	te.event_type,
	te.event_time,
	te.latitude,
	te.longitude,
	te.actor_user_id,
	te.notes,
	COALESCE(TRIM(CONCAT_WS(' ', au.first_name, au.last_name, au.other_name)),'')
FROM trip_events te
LEFT JOIN users au ON au.id = te.actor_user_id
WHERE te.trip_id = $1
ORDER BY te.event_time DESC
LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, query, tripID, p.PageSize, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.TripEvent, 0)
	for rows.Next() {
		var e domain.TripEvent
		if err := rows.Scan(
			&e.ID,
			&e.TripID,
			&e.EventType,
			&e.EventTime,
			&e.Latitude,
			&e.Longitude,
			&e.ActorUserID,
			&e.Notes,
			&e.ActorUserName,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, e)
	}
	return items, total, rows.Err()
}

func (r *Repository) CreateTripEvent(ctx context.Context, tripID string, in domain.TripEvent) (domain.TripEvent, error) {
	const q = `
INSERT INTO trip_events (
	id, trip_id, event_type, event_time, latitude, longitude, actor_user_id, notes
) VALUES (
	$1,$2,$3,$4,$5,$6,$7,$8
)
RETURNING event_time`
	if err := r.db.QueryRow(
		ctx,
		q,
		in.ID,
		tripID,
		in.EventType,
		in.EventTime,
		in.Latitude,
		in.Longitude,
		in.ActorUserID,
		in.Notes,
	).Scan(&in.EventTime); err != nil {
		return domain.TripEvent{}, err
	}
	return in, nil
}
