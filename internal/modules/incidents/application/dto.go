package application

import (
	"dispatch/internal/modules/incidents/domain"
	platformdb "dispatch/internal/platform/db"
	"time"
)

type TriageResponseInput struct {
	QuestionCode  string `json:"question_code" binding:"required"`
	ResponseValue string `json:"response_value" binding:"required"`
}

type CreateIncidentRequest struct {
	SourceChannel     string                `json:"source_channel" binding:"required"`
	CallerName        string                `json:"caller_name"`
	CallerPhone       string                `json:"caller_phone"`
	PatientName       string                `json:"patient_name"`
	PatientPhone      string                `json:"patient_phone"`
	PatientAgeGroup   string                `json:"patient_age_group"`
	PatientSex        string                `json:"patient_sex"`
	IncidentTypeID    string                `json:"incident_type_id" binding:"required,uuid"`
	SeverityLevelID   *string               `json:"severity_level_id"`
	PriorityLevelID   *string               `json:"priority_level_id"`
	Summary           string                `json:"summary" binding:"required"`
	Description       string                `json:"description"`
	DistrictID        *string               `json:"district_id"`
	FacilityID        *string               `json:"facility_id"`
	Village           string                `json:"village"`
	Parish            string                `json:"parish"`
	Subcounty         string                `json:"subcounty"`
	Landmark          string                `json:"landmark"`
	Latitude          *float64              `json:"latitude"`
	Longitude         *float64              `json:"longitude"`
	CreatedByUserID   *string               `json:"created_by_user_id"`
	QuestionnaireCode string                `json:"questionnaire_code"`
	TriageResponses   []TriageResponseInput `json:"triage_responses"`
	TriageNotes       string                `json:"triage_notes"`
}

type UpdateIncidentStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Notes  string `json:"notes"`
}

type UpdateIncidentRequest struct {
	SourceChannel      *string               `json:"source_channel,omitempty" binding:"omitempty,oneof=SMS USSD CALL MOBILE_APP WEB_PORTAL FACILITY_REFERRAL"`
	CallerName         *string               `json:"caller_name,omitempty"`
	CallerPhone        *string               `json:"caller_phone,omitempty"`
	PatientName        *string               `json:"patient_name,omitempty"`
	PatientPhone       *string               `json:"patient_phone,omitempty"`
	PatientAgeGroup    *string               `json:"patient_age_group,omitempty"`
	PatientSex         *string               `json:"patient_sex,omitempty" binding:"omitempty,oneof=MALE FEMALE OTHER UNKNOWN"`
	IncidentTypeID     *string               `json:"incident_type_id,omitempty" binding:"omitempty,uuid"`
	SeverityLevelID    *string               `json:"severity_level_id,omitempty" binding:"omitempty,uuid"`
	PriorityLevelID    *string               `json:"priority_level_id,omitempty" binding:"omitempty,uuid"`
	Summary            *string               `json:"summary,omitempty"`
	Description        *string               `json:"description,omitempty"`
	DistrictID         *string               `json:"district_id,omitempty" binding:"omitempty,uuid"`
	FacilityID         *string               `json:"facility_id,omitempty" binding:"omitempty,uuid"`
	Village            *string               `json:"village,omitempty"`
	Parish             *string               `json:"parish,omitempty"`
	Subcounty          *string               `json:"subcounty,omitempty"`
	Landmark           *string               `json:"landmark,omitempty"`
	Latitude           *float64              `json:"latitude,omitempty"`
	Longitude          *float64              `json:"longitude,omitempty"`
	VerificationStatus *string               `json:"verification_status,omitempty" binding:"omitempty,oneof=PENDING VERIFIED REJECTED"`
	Status             *string               `json:"status,omitempty" binding:"omitempty,oneof=NEW PENDING_VERIFICATION VERIFIED AWAITING_ASSIGNMENT ASSIGNED ENROUTE AT_SCENE TRANSPORTING COMPLETED CANCELLED ESCALATED REJECTED"`
	ReportedAt         *time.Time            `json:"reported_at,omitempty"`
	QuestionnaireCode  string                `json:"questionnaire_code,omitempty"`
	TriageResponses    []TriageResponseInput `json:"triage_responses,omitempty"`
	TriageNotes        string                `json:"triage_notes,omitempty"`
	Notes              string                `json:"notes,omitempty"`
}

type ListIncidentsParams struct {
	Status     *string               `json:"status,omitempty"`
	DistrictID *string               `json:"district_id,omitempty"`
	FacilityID *string               `json:"facility_id,omitempty"`
	PriorityID *string               `json:"priority_id,omitempty"`
	Pagination platformdb.Pagination `json:"pagination"`
}

type CreateIncidentResponse struct {
	Incident                   domain.Incident                `json:"incident"`
	TriageSession              *domain.PersistedTriageSession `json:"triage_session,omitempty"`
	AutoDispatchEligible       bool                           `json:"auto_dispatch_eligible"`
	DispatchRecommendationHint string                         `json:"dispatch_recommendation_hint"`
}

type UpdateIncidentResponse struct {
	Incident                   domain.Incident                `json:"incident"`
	TriageSession              *domain.PersistedTriageSession `json:"triage_session,omitempty"`
	AutoDispatchEligible       bool                           `json:"auto_dispatch_eligible"`
	DispatchRecommendationHint string                         `json:"dispatch_recommendation_hint"`
}
