package domain

import "time"

type DispatchAssignment struct {
	ID                   string     `json:"id"`
	IncidentID           string     `json:"incident_id"`
	AmbulanceID          *string    `json:"ambulance_id,omitempty"`
	AssignedByUserID     *string    `json:"assigned_by_user_id,omitempty"`
	DriverUserID         *string    `json:"driver_user_id,omitempty"`
	LeadMedicUserID      *string    `json:"lead_medic_user_id,omitempty"`
	TeamSnapshotJSON     []byte     `json:"team_snapshot_json,omitempty"`
	AssignmentMode       string     `json:"assignment_mode"`
	RankingScore         *float64   `json:"ranking_score,omitempty"`
	ETAMinutes           *int       `json:"eta_minutes,omitempty"`
	Status               string     `json:"status"`
	AssignedAt           *time.Time `json:"assigned_at,omitempty"`
	AcceptedAt           *time.Time `json:"accepted_at,omitempty"`
	DepartedAt           *time.Time `json:"departed_at,omitempty"`
	ArrivedSceneAt       *time.Time `json:"arrived_scene_at,omitempty"`
	PatientLoadedAt      *time.Time `json:"patient_loaded_at,omitempty"`
	ArrivedDestinationAt *time.Time `json:"arrived_destination_at,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
	CancelledAt          *time.Time `json:"cancelled_at,omitempty"`
	CancellationReason   string     `json:"cancellation_reason"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type DispatchAssignmentResponse struct {
	ID             string `json:"id"`
	IncidentID     string `json:"incident_id"`
	IncidentNumber string `json:"incident_number"`

	AmbulanceID       *string `json:"ambulance_id,omitempty"`
	AmbulanceCode     string  `json:"ambulance_code,omitempty"`
	PlateNumber       string  `json:"plate_number,omitempty"`
	AmbulanceCategory string  `json:"ambulance_category,omitempty"`

	AssignedByUserID *string `json:"assigned_by_user_id,omitempty"`
	AssignedByName   string  `json:"assigned_by_name,omitempty"`
	DriverUserID     *string `json:"driver_user_id,omitempty"`
	DriverName       string  `json:"driver_name,omitempty"`
	LeadMedicUserID  *string `json:"lead_medic_user_id,omitempty"`
	LeadMedicName    string  `json:"lead_medic_name,omitempty"`

	AssignmentMode string   `json:"assignment_mode"`
	RankingScore   *float64 `json:"ranking_score,omitempty"`
	ETAMinutes     *int     `json:"eta_minutes,omitempty"`
	Status         string   `json:"status"`

	AssignedAt           *time.Time `json:"assigned_at,omitempty"`
	AcceptedAt           *time.Time `json:"accepted_at,omitempty"`
	DepartedAt           *time.Time `json:"departed_at,omitempty"`
	ArrivedSceneAt       *time.Time `json:"arrived_scene_at,omitempty"`
	PatientLoadedAt      *time.Time `json:"patient_loaded_at,omitempty"`
	ArrivedDestinationAt *time.Time `json:"arrived_destination_at,omitempty"`
	CompletedAt          *time.Time `json:"completed_at,omitempty"`
	CancelledAt          *time.Time `json:"cancelled_at,omitempty"`

	CancellationReason string    `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type DispatchRecommendation struct {
	ID           string    `json:"id"`
	IncidentID   string    `json:"incident_id"`
	AmbulanceID  string    `json:"ambulance_id"`
	DriverUserID *string   `json:"driver_user_id,omitempty"`
	Score        float64   `json:"score"`
	ETAMinutes   *int      `json:"eta_minutes,omitempty"`
	RuleSummary  string    `json:"rule_summary"`
	GeneratedAt  time.Time `json:"generated_at"`
	Selected     bool      `json:"selected"`

	// Derived display names resolved via joins (omitted when blank).
	AmbulanceCode  string `json:"ambulance_code,omitempty"`
	AmbulancePlate string `json:"plate_number,omitempty"`
	DriverName     string `json:"driver_name,omitempty"`
}

type TriageEvaluation struct {
	QuestionCode  string `json:"question_code"`
	ResponseValue string `json:"response_value"`
}

type PersistedTriageResponse struct {
	QuestionID         string  `json:"question_id"`
	QuestionCode       string  `json:"question_code"`
	ResponseType       string  `json:"response_type"`
	ResponseValueText  *string `json:"response_value_text,omitempty"`
	ResponseValueBool  *bool   `json:"response_value_bool,omitempty"`
	ResponseValueInt   *int    `json:"response_value_int,omitempty"`
	SelectedOptionID   *string `json:"selected_option_id,omitempty"`
	SelectedOptionCode *string `json:"selected_option_code,omitempty"`
	ScoreAwarded       int     `json:"score_awarded"`
}

type PersistedTriageSession struct {
	ID                     string                    `json:"id"`
	IncidentID             string                    `json:"incident_id"`
	QuestionnaireID        string                    `json:"questionnaire_id"`
	TriageMode             string                    `json:"triage_mode"`
	TotalScore             int                       `json:"total_score"`
	BooleanTrueCount       int                       `json:"boolean_true_count"`
	AutoDispatchEligible   bool                      `json:"auto_dispatch_eligible"`
	DerivedPriorityLevelID *string                   `json:"derived_priority_level_id,omitempty"`
	DerivedPriorityCode    string                    `json:"derived_priority_code,omitempty"`
	Notes                  string                    `json:"notes"`
	TriagedByUserID        *string                   `json:"triaged_by_user_id,omitempty"`
	TriagedAt              time.Time                 `json:"triaged_at"`
	CreatedAt              time.Time                 `json:"created_at"`
	UpdatedAt              time.Time                 `json:"updated_at"`
	Responses              []PersistedTriageResponse `json:"responses,omitempty"`
}

type IncidentDispatchContext struct {
	IncidentID         string   `json:"incident_id"`
	PriorityLevelID    *string  `json:"priority_level_id,omitempty"`
	PriorityCode       string   `json:"priority_code,omitempty"`
	DistrictID         *string  `json:"district_id,omitempty"`
	FacilityID         *string  `json:"facility_id,omitempty"`
	Latitude           *float64 `json:"latitude,omitempty"`
	Longitude          *float64 `json:"longitude,omitempty"`
	VerificationStatus string   `json:"verification_status"`
	Status             string   `json:"status"`
}

type AmbulanceCandidate struct {
	AmbulanceID  string   `json:"ambulance_id"`
	DriverUserID *string  `json:"driver_user_id,omitempty"`
	DistrictID   *string  `json:"district_id,omitempty"`
	FacilityID   *string  `json:"facility_id,omitempty"`
	Latitude     *float64 `json:"latitude,omitempty"`
	Longitude    *float64 `json:"longitude,omitempty"`
	Availability string   `json:"availability"`
	Dispatchable bool     `json:"dispatchable"`
	ETAMinutes   *int     `json:"eta_minutes,omitempty"`
}

type QuestionDefinition struct {
	QuestionID   string
	ResponseType string
	TrueScore    *int
	FalseScore   *int
}
