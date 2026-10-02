package domain

import "time"

type Trip struct {
	ID                    string     `json:"id"`
	DispatchAssignmentID  string     `json:"dispatch_assignment_id"`
	IncidentID            string     `json:"incident_id"`
	AmbulanceID           *string    `json:"ambulance_id,omitempty"`
	OriginLat             *float64   `json:"origin_lat,omitempty"`
	OriginLon             *float64   `json:"origin_lon,omitempty"`
	SceneLat              *float64   `json:"scene_lat,omitempty"`
	SceneLon              *float64   `json:"scene_lon,omitempty"`
	DestinationFacilityID *string    `json:"destination_facility_id,omitempty"`
	DestinationLat        *float64   `json:"destination_lat,omitempty"`
	DestinationLon        *float64   `json:"destination_lon,omitempty"`
	OdometerStart         *float64   `json:"odometer_start,omitempty"`
	OdometerEnd           *float64   `json:"odometer_end,omitempty"`
	StartedAt             *time.Time `json:"started_at,omitempty"`
	EndedAt               *time.Time `json:"ended_at,omitempty"`
	Outcome               *string    `json:"outcome,omitempty"`
	Notes                 *string    `json:"notes,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`

	// Derived display names resolved via joins (omitted when blank).
	IncidentNumber          string `json:"incident_number,omitempty"`
	AmbulanceCode           string `json:"ambulance_code,omitempty"`
	AmbulancePlate          string `json:"ambulance_plate,omitempty"`
	DestinationFacilityName string `json:"destination_facility_name,omitempty"`

	// Context from the linked incident and dispatch assignment.
	IncidentType           string     `json:"incident_type,omitempty"`
	IncidentSummary        string     `json:"incident_summary,omitempty"`
	SceneLocation          string     `json:"scene_location,omitempty"`
	PlannedDestinationName string     `json:"planned_destination_name,omitempty"`
	PriorityName           string     `json:"priority_name,omitempty"`
	PriorityRank           *int       `json:"priority_rank,omitempty"`
	AmbulanceMake          string     `json:"ambulance_make,omitempty"`
	AmbulanceModel         string     `json:"ambulance_model,omitempty"`
	DriverName             string     `json:"driver_name,omitempty"`
	DriverPhone            string     `json:"driver_phone,omitempty"`
	LeadMedicName          string     `json:"lead_medic_name,omitempty"`
	LeadMedicPhone         string     `json:"lead_medic_phone,omitempty"`
	AssignmentStatus       string     `json:"assignment_status,omitempty"`
	AssignedAt             *time.Time `json:"assigned_at,omitempty"`
	DepartedAt             *time.Time `json:"departed_at,omitempty"`
	ArrivedSceneAt         *time.Time `json:"arrived_scene_at,omitempty"`
	PatientLoadedAt        *time.Time `json:"patient_loaded_at,omitempty"`
	ArrivedDestinationAt   *time.Time `json:"arrived_destination_at,omitempty"`
}

// TripSummary aggregates the whole trip registry for dashboards.
type TripSummary struct {
	Total     int64 `json:"total"`
	Active    int64 `json:"active"`
	Completed int64 `json:"completed"`
	Cancelled int64 `json:"cancelled"`
	// StartedLast24h counts trips whose started_at falls in the last 24 hours.
	StartedLast24h     int64    `json:"started_last_24h"`
	DistanceKm         float64  `json:"distance_km"`
	AvgDurationMinutes *float64 `json:"avg_duration_minutes,omitempty"`
}

type TripEvent struct {
	ID          string    `json:"id"`
	TripID      string    `json:"trip_id"`
	EventType   string    `json:"event_type"`
	EventTime   time.Time `json:"event_time"`
	Latitude    *float64  `json:"latitude,omitempty"`
	Longitude   *float64  `json:"longitude,omitempty"`
	ActorUserID *string   `json:"actor_user_id,omitempty"`
	Notes       *string   `json:"notes,omitempty"`

	// Derived display name resolved via join (omitted when blank).
	ActorUserName string `json:"actor_user_name,omitempty"`
}
