package application

type CreateTripRequest struct {
	DispatchAssignmentID  string   `json:"dispatch_assignment_id" binding:"required"`
	IncidentID            string   `json:"incident_id" binding:"required"`
	AmbulanceID           *string  `json:"ambulance_id,omitempty"`
	OriginLat             *float64 `json:"origin_lat,omitempty"`
	OriginLon             *float64 `json:"origin_lon,omitempty"`
	SceneLat              *float64 `json:"scene_lat,omitempty"`
	SceneLon              *float64 `json:"scene_lon,omitempty"`
	DestinationFacilityID *string  `json:"destination_facility_id,omitempty"`
	DestinationLat        *float64 `json:"destination_lat,omitempty"`
	DestinationLon        *float64 `json:"destination_lon,omitempty"`
	OdometerStart         *float64 `json:"odometer_start,omitempty"`
	OdometerEnd           *float64 `json:"odometer_end,omitempty"`
	StartedAt             *string  `json:"started_at,omitempty"`
	EndedAt               *string  `json:"ended_at,omitempty"`
	Outcome               *string  `json:"outcome,omitempty"`
	Notes                 *string  `json:"notes,omitempty"`
}

type UpdateTripRequest struct {
	AmbulanceID           *string  `json:"ambulance_id,omitempty"`
	OriginLat             *float64 `json:"origin_lat,omitempty"`
	OriginLon             *float64 `json:"origin_lon,omitempty"`
	SceneLat              *float64 `json:"scene_lat,omitempty"`
	SceneLon              *float64 `json:"scene_lon,omitempty"`
	DestinationFacilityID *string  `json:"destination_facility_id,omitempty"`
	DestinationLat        *float64 `json:"destination_lat,omitempty"`
	DestinationLon        *float64 `json:"destination_lon,omitempty"`
	OdometerStart         *float64 `json:"odometer_start,omitempty"`
	OdometerEnd           *float64 `json:"odometer_end,omitempty"`
	StartedAt             *string  `json:"started_at,omitempty"`
	EndedAt               *string  `json:"ended_at,omitempty"`
	Outcome               *string  `json:"outcome,omitempty"`
	Notes                 *string  `json:"notes,omitempty"`
}

type CreateTripEventRequest struct {
	EventType   string   `json:"event_type" binding:"required" enums:"ASSIGNED,ACCEPTED,DEPARTED,ARRIVED_SCENE,PATIENT_LOADED,ARRIVED_DESTINATION,COMPLETED,CANCELLED,GPS_PING,NOTE"`
	EventTime   *string  `json:"event_time,omitempty"`
	Latitude    *float64 `json:"latitude,omitempty"`
	Longitude   *float64 `json:"longitude,omitempty"`
	ActorUserID *string  `json:"actor_user_id,omitempty"`
	Notes       *string  `json:"notes,omitempty"`
}
