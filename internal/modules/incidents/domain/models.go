package domain

import "time"

type Incident struct {
	ID                      string     `json:"id"`
	IncidentNumber          string     `json:"incident_number"`
	SourceChannel           string     `json:"source_channel"`
	CallerName              string     `json:"caller_name"`
	CallerPhone             string     `json:"caller_phone"`
	PatientName             string     `json:"patient_name"`
	PatientPhone            string     `json:"patient_phone"`
	PatientAgeGroup         string     `json:"patient_age_group"`
	PatientSex              string     `json:"patient_sex"`
	PatientDetailsDiagnosis string     `json:"patient_details_diagnosis"`
	RespiratoryRate         string     `json:"respiratory_rate"`
	Spo2                    string     `json:"spo2"`
	Pulse                   string     `json:"pulse"`
	BP                      string     `json:"bp"`
	Temperature             string     `json:"temperature"`
	IncidentTypeID          string     `json:"incident_type_id"`
	SeverityLevelID         *string    `json:"severity_level_id,omitempty"`
	PriorityLevelID         *string    `json:"priority_level_id,omitempty"`
	PriorityCode            string     `json:"priority_code,omitempty"`
	Summary                 string     `json:"summary"`
	Description             string     `json:"description"`
	DistrictID              *string    `json:"district_id,omitempty"`
	PickupLocation          string     `json:"pickup_location"`
	ReceivingFacilityID     *string    `json:"receiving_facility_id,omitempty"`
	ReferringFacilityID     *string    `json:"referring_facility_id,omitempty"`
	Village                 string     `json:"village"`
	Parish                  string     `json:"parish"`
	Subcounty               string     `json:"subcounty"`
	Landmark                string     `json:"landmark"`
	Latitude                *float64   `json:"latitude,omitempty"`
	Longitude               *float64   `json:"longitude,omitempty"`
	VerificationStatus      string     `json:"verification_status"`
	Status                  string     `json:"status"`
	ReportedAt              time.Time  `json:"reported_at"`
	CreatedByUserID         *string    `json:"created_by_user_id,omitempty"`
	TriagedByUserID         *string    `json:"triaged_by_user_id,omitempty"`
	TriagedAt               *time.Time `json:"triaged_at,omitempty"`
	AssignedAt              *time.Time `json:"assigned_at,omitempty"`
	ClosedAt                *time.Time `json:"closed_at,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type IncidentUpdate struct {
	ID          string    `json:"id"`
	IncidentID  string    `json:"incident_id"`
	UpdateType  string    `json:"update_type"`
	OldValue    string    `json:"old_value"`
	NewValue    string    `json:"new_value"`
	Notes       string    `json:"notes"`
	ActorUserID *string   `json:"actor_user_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
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
	QuestionnaireCode      string                    `json:"questionnaire_code"`
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
