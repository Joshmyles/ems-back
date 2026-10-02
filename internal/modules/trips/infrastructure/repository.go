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

// Trip state is derived: outcome is free text, so "cancel"/"complete" in it
// (or an end time) decide where a trip sits.
const (
	tripCancelledSQL = `COALESCE(t.outcome,'') ILIKE '%cancel%'`
	tripCompletedSQL = `(NOT ` + tripCancelledSQL + ` AND (t.ended_at IS NOT NULL OR COALESCE(t.outcome,'') ILIKE '%complete%'))`
	tripActiveSQL    = `(NOT ` + tripCancelledSQL + ` AND t.ended_at IS NULL AND COALESCE(t.outcome,'') NOT ILIKE '%complete%')`
)

// tripFromSQL joins everything the registry shows next to a trip.
const tripFromSQL = `
FROM trips t
LEFT JOIN incidents i ON i.id = t.incident_id
LEFT JOIN ref_incident_types rit ON rit.id = i.incident_type_id
LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
LEFT JOIN ref_districts rd ON rd.id = i.district_id
LEFT JOIN ref_facilities rcf ON rcf.id = i.receiving_facility_id
LEFT JOIN ambulances a ON a.id = t.ambulance_id
LEFT JOIN ref_facilities df ON df.id = t.destination_facility_id
LEFT JOIN dispatch_assignments da ON da.id = t.dispatch_assignment_id
LEFT JOIN users du ON du.id = da.driver_user_id
LEFT JOIN users lmu ON lmu.id = da.lead_medic_user_id`

// tripSelectSQL is shared by list and get. Column order must match scanTrip.
const tripSelectSQL = `
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
	COALESCE(df.name,''),
	COALESCE(rit.name,''),
	COALESCE(i.summary,''),
	CONCAT_WS(', ',
		NULLIF(TRIM(i.landmark),''),
		NULLIF(TRIM(i.village),''),
		NULLIF(TRIM(i.subcounty),''),
		NULLIF(TRIM(rd.name),'')
	),
	COALESCE(rcf.name,''),
	COALESCE(rpl.name,''),
	rpl.sort_order,
	COALESCE(a.make,''),
	COALESCE(a.model,''),
	COALESCE(TRIM(CONCAT_WS(' ', du.first_name, du.last_name, du.other_name)),''),
	COALESCE(du.phone,''),
	COALESCE(TRIM(CONCAT_WS(' ', lmu.first_name, lmu.last_name, lmu.other_name)),''),
	COALESCE(lmu.phone,''),
	COALESCE(da.status,''),
	da.assigned_at,
	da.departed_at,
	da.arrived_scene_at,
	da.patient_loaded_at,
	da.arrived_destination_at
` + tripFromSQL

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTrip(row rowScanner) (domain.Trip, error) {
	var t domain.Trip
	err := row.Scan(
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
		&t.IncidentType,
		&t.IncidentSummary,
		&t.SceneLocation,
		&t.PlannedDestinationName,
		&t.PriorityName,
		&t.PriorityRank,
		&t.AmbulanceMake,
		&t.AmbulanceModel,
		&t.DriverName,
		&t.DriverPhone,
		&t.LeadMedicName,
		&t.LeadMedicPhone,
		&t.AssignmentStatus,
		&t.AssignedAt,
		&t.DepartedAt,
		&t.ArrivedSceneAt,
		&t.PatientLoadedAt,
		&t.ArrivedDestinationAt,
	)
	return t, err
}

func (r *Repository) ListTrips(ctx context.Context, p platformdb.Pagination) ([]domain.Trip, int64, error) {
	allowedSorts := map[string]string{
		"started_at": "t.started_at",
		"created_at": "t.created_at",
	}
	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			COALESCE(i.incident_number,'') ILIKE $%[1]d OR
			COALESCE(a.code,'') ILIKE $%[1]d OR
			COALESCE(a.plate_number,'') ILIKE $%[1]d OR
			COALESCE(df.name,'') ILIKE $%[1]d OR
			COALESCE(t.outcome,'') ILIKE $%[1]d OR
			COALESCE(t.notes,'') ILIKE $%[1]d OR
			CONCAT_WS(' ', du.first_name, du.last_name, lmu.first_name, lmu.last_name) ILIKE $%[1]d
		)`, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

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
		case "state":
			switch strings.ToLower(value) {
			case "active":
				where = append(where, tripActiveSQL)
			case "completed":
				where = append(where, tripCompletedSQL)
			case "cancelled":
				where = append(where, tripCancelledSQL)
			}
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	countSQL := `SELECT COUNT(1) ` + tripFromSQL + ` ` + whereSQL
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)
	listSQL := fmt.Sprintf(`%s
%s
%s
LIMIT $%d OFFSET $%d`, tripSelectSQL, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, listSQL, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.Trip, 0)
	for rows.Next() {
		t, err := scanTrip(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, t)
	}
	return items, total, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string) (domain.Trip, error) {
	return scanTrip(r.db.QueryRow(ctx, tripSelectSQL+` WHERE t.id = $1`, id))
}

func (r *Repository) SummarizeTrips(ctx context.Context) (domain.TripSummary, error) {
	var out domain.TripSummary
	err := r.db.QueryRow(ctx, `
		SELECT
			COUNT(1),
			COUNT(1) FILTER (WHERE `+tripActiveSQL+`),
			COUNT(1) FILTER (WHERE `+tripCompletedSQL+`),
			COUNT(1) FILTER (WHERE `+tripCancelledSQL+`),
			COUNT(1) FILTER (WHERE t.started_at >= now() - interval '24 hours'),
			COALESCE(SUM(t.odometer_end - t.odometer_start)
				FILTER (WHERE t.odometer_start IS NOT NULL AND t.odometer_end IS NOT NULL), 0)::float8,
			(AVG(EXTRACT(EPOCH FROM t.ended_at - t.started_at) / 60)
				FILTER (WHERE t.started_at IS NOT NULL AND t.ended_at >= t.started_at))::float8
		FROM trips t`).Scan(
		&out.Total, &out.Active, &out.Completed, &out.Cancelled,
		&out.StartedLast24h, &out.DistanceKm, &out.AvgDurationMinutes,
	)
	return out, err
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
