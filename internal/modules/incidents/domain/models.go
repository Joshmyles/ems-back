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
	CasualtyCount           *int       `json:"casualty_count,omitempty"`
	IncidentTypeID          string     `json:"incident_type_id"`
	IncidentTypeName        string     `json:"incident_type_name,omitempty"`
	SeverityLevelID         *string    `json:"severity_level_id,omitempty"`
	SeverityName            string     `json:"severity_name,omitempty"`
	PriorityLevelID         *string    `json:"priority_level_id,omitempty"`
	PriorityCode            string     `json:"priority_code,omitempty"`
	PriorityName            string     `json:"priority_name,omitempty"`
	Summary                 string     `json:"summary"`
	Description             string     `json:"description"`
	DistrictID              *string    `json:"district_id,omitempty"`
	DistrictName            string     `json:"district_name,omitempty"`
	PickupLocation          string     `json:"pickup_location"`
	ReceivingFacilityID     *string    `json:"receiving_facility_id,omitempty"`
	ReceivingFacilityName   string     `json:"receiving_facility_name,omitempty"`
	ReferringFacilityID     *string    `json:"referring_facility_id,omitempty"`
	ReferringFacilityName   string     `json:"referring_facility_name,omitempty"`
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

// TriageAnswer is one question/answer pair from a completed triage session,
// with the question text resolved for display.
type TriageAnswer struct {
	QuestionCode string `json:"question_code"`
	QuestionText string `json:"question_text"`
	ResponseType string `json:"response_type"`
	Answer       string `json:"answer"`
	ScoreAwarded int    `json:"score_awarded"`
}

// TriageInfo is the read model for an incident's latest triage session.
type TriageInfo struct {
	QuestionnaireName    string         `json:"questionnaire_name"`
	TriageMode           string         `json:"triage_mode"`
	TotalScore           int            `json:"total_score"`
	AutoDispatchEligible bool           `json:"auto_dispatch_eligible"`
	DerivedPriorityCode  string         `json:"derived_priority_code,omitempty"`
	Notes                string         `json:"notes"`
	TriagedAt            time.Time      `json:"triaged_at"`
	TriagedByName        string         `json:"triaged_by_name,omitempty"`
	Answers              []TriageAnswer `json:"answers"`
}

// IncidentFeedback is a receiving-facility outcome report recorded against an
// incident (e.g. the patient was admitted, discharged, referred onward, etc.).
type IncidentFeedback struct {
	ID              string    `json:"id"`
	IncidentID      string    `json:"incident_id"`
	OutcomeStatus   string    `json:"outcome_status"`
	Summary         string    `json:"summary"`
	ReportedBy      string    `json:"reported_by,omitempty"`
	OtherDetails    string    `json:"other_details,omitempty"`
	CreatedByUserID *string   `json:"created_by_user_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
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
