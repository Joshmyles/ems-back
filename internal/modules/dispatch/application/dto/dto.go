package dto

import platformdb "dispatch/internal/platform/db"

type EvaluateDispatchRequest struct {
	IncidentID string `json:"incident_id" binding:"required,uuid"`
	Responses  []struct {
		QuestionCode  string `json:"question_code" binding:"required"`
		ResponseValue string `json:"response_value" binding:"required"`
	} `json:"responses" binding:"required,dive"`
}

type PersistTriageRequest struct {
	IncidentID        string `json:"incident_id" binding:"required,uuid"`
	QuestionnaireCode string `json:"questionnaire_code" binding:"required"`
	TriageMode        string `json:"triage_mode"`
	Notes             string `json:"notes"`
	Responses         []struct {
		QuestionCode  string `json:"question_code" binding:"required"`
		ResponseValue string `json:"response_value" binding:"required"`
	} `json:"responses" binding:"required,dive"`
}

type GenerateRecommendationsRequest struct {
	IncidentID string `json:"incident_id" binding:"required,uuid"`
	Auto       bool   `json:"auto"`
}

type CreateDispatchAssignmentRequest struct {
	IncidentID       string   `json:"incident_id" binding:"required,uuid"`
	AmbulanceID      *string  `json:"ambulance_id"`
	AssignedByUserID *string  `json:"assigned_by_user_id"`
	DriverUserID     *string  `json:"driver_user_id"`
	LeadMedicUserID  *string  `json:"lead_medic_user_id"`
	AssignmentMode   string   `json:"assignment_mode"`
	RankingScore     *float64 `json:"ranking_score"`
	ETAMinutes       *int     `json:"eta_minutes"`
}

type UpdateDispatchStatusRequest struct {
	Status             string `json:"status" binding:"required"`
	CancellationReason string `json:"cancellation_reason"`
}

type ListAssignmentsParams struct {
	IncidentID  *string `json:"incident_id,omitempty"`
	AmbulanceID *string `json:"ambulance_id,omitempty"`
	Status      *string `json:"status,omitempty"`
	// Active restricts results to assignments still engaged on a case
	// (not completed, cancelled or declined).
	Active     bool                  `json:"active,omitempty"`
	Pagination platformdb.Pagination `json:"pagination"`
}

type ListRecommendationsParams struct {
	IncidentID string                `json:"incident_id"`
	Pagination platformdb.Pagination `json:"pagination"`
}
