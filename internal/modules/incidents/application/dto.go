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
	SourceChannel           string                `json:"source_channel"`
	CallerName              string                `json:"caller_name"`
	CallerPhone             string                `json:"caller_phone"`
	PatientName             string                `json:"patient_name"`
	PatientPhone            string                `json:"patient_phone"`
	PatientAgeGroup         string                `json:"patient_age_group"`
	PatientSex              string                `json:"patient_sex"`
	PatientDetailsDiagnosis string                `json:"patient_details_diagnosis"`
	RespiratoryRate         string                `json:"respiratory_rate"`
	Spo2                    string                `json:"spo2"`
	Pulse                   string                `json:"pulse"`
	BP                      string                `json:"bp"`
	Temperature             string                `json:"temperature"`
	IncidentTypeID          string                `json:"incident_type_id" binding:"omitempty,uuid"`
	SeverityLevelID         *string               `json:"severity_level_id"`
	PriorityLevelID         *string               `json:"priority_level_id"`
	Summary                 string                `json:"summary"`
	Description             string                `json:"description"`
	DistrictID              *string               `json:"district_id"`
	PickupLocation          string                `json:"pickup_location" binding:"omitempty,oneof=COMMUNITY FACILITY"`
	ReceivingFacilityID     *string               `json:"receiving_facility_id" binding:"omitempty,uuid"`
	ReferringFacilityID     *string               `json:"referring_facility_id" binding:"omitempty,uuid"`
	Village                 string                `json:"village"`
	Parish                  string                `json:"parish"`
	Subcounty               string                `json:"subcounty"`
	Landmark                string                `json:"landmark"`
	Latitude                *float64              `json:"latitude"`
	Longitude               *float64              `json:"longitude"`
	CreatedByUserID         *string               `json:"created_by_user_id"`
	QuestionnaireCode       string                `json:"questionnaire_code"`
	TriageResponses         []TriageResponseInput `json:"triage_responses"`
	TriageNotes             string                `json:"triage_notes"`
}

type UpdateIncidentStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=NEW PENDING_VERIFICATION VERIFIED AWAITING_ASSIGNMENT ASSIGNED ENROUTE AT_SCENE TRANSPORTING COMPLETED CANCELLED ESCALATED REJECTED"`
	Notes  string `json:"notes"`
}

type UpdateIncidentRequest struct {
	SourceChannel           *string               `json:"source_channel,omitempty" binding:"omitempty,oneof=SMS USSD CALL MOBILE_APP WEB_PORTAL FACILITY_REFERRAL"`
	CallerName              *string               `json:"caller_name,omitempty"`
	CallerPhone             *string               `json:"caller_phone,omitempty"`
	PatientName             *string               `json:"patient_name,omitempty"`
	PatientPhone            *string               `json:"patient_phone,omitempty"`
	PatientAgeGroup         *string               `json:"patient_age_group,omitempty"`
	PatientSex              *string               `json:"patient_sex,omitempty" binding:"omitempty,oneof=MALE FEMALE OTHER UNKNOWN"`
	PatientDetailsDiagnosis *string               `json:"patient_details_diagnosis,omitempty"`
	RespiratoryRate         *string               `json:"respiratory_rate,omitempty"`
	Spo2                    *string               `json:"spo2,omitempty"`
	Pulse                   *string               `json:"pulse,omitempty"`
	BP                      *string               `json:"bp,omitempty"`
	Temperature             *string               `json:"temperature,omitempty"`
	IncidentTypeID          *string               `json:"incident_type_id,omitempty" binding:"omitempty,uuid"`
	SeverityLevelID         *string               `json:"severity_level_id,omitempty" binding:"omitempty,uuid"`
	PriorityLevelID         *string               `json:"priority_level_id,omitempty" binding:"omitempty,uuid"`
	Summary                 *string               `json:"summary,omitempty"`
	Description             *string               `json:"description,omitempty"`
	DistrictID              *string               `json:"district_id,omitempty" binding:"omitempty,uuid"`
	PickupLocation          *string               `json:"pickup_location,omitempty" binding:"omitempty,oneof=COMMUNITY FACILITY"`
	ReceivingFacilityID     *string               `json:"receiving_facility_id,omitempty" binding:"omitempty,uuid"`
	ReferringFacilityID     *string               `json:"referring_facility_id,omitempty" binding:"omitempty,uuid"`
	Village                 *string               `json:"village,omitempty"`
	Parish                  *string               `json:"parish,omitempty"`
	Subcounty               *string               `json:"subcounty,omitempty"`
	Landmark                *string               `json:"landmark,omitempty"`
	Latitude                *float64              `json:"latitude,omitempty"`
	Longitude               *float64              `json:"longitude,omitempty"`
	VerificationStatus      *string               `json:"verification_status,omitempty" binding:"omitempty,oneof=PENDING VERIFIED REJECTED"`
	Status                  *string               `json:"status,omitempty" binding:"omitempty,oneof=NEW PENDING_VERIFICATION VERIFIED AWAITING_ASSIGNMENT ASSIGNED ENROUTE AT_SCENE TRANSPORTING COMPLETED CANCELLED ESCALATED REJECTED"`
	ReportedAt              *time.Time            `json:"reported_at,omitempty"`
	QuestionnaireCode       string                `json:"questionnaire_code,omitempty"`
	TriageResponses         []TriageResponseInput `json:"triage_responses,omitempty"`
	TriageNotes             string                `json:"triage_notes,omitempty"`
	Notes                   string                `json:"notes,omitempty"`
}

// CreateIncidentFeedbackRequest is the receiving facility's outcome report for a
// transferred/received patient.
type CreateIncidentFeedbackRequest struct {
	OutcomeStatus string `json:"outcome_status" binding:"required,oneof=ADMITTED DISCHARGED STABILIZED REFERRED DECEASED OTHER"`
	Summary       string `json:"summary" binding:"required"`
	ReportedBy    string `json:"reported_by"`
	OtherDetails  string `json:"other_details"`
}

type ListIncidentsParams struct {
	Status              *string    `json:"status,omitempty"`
	DistrictID          *string    `json:"district_id,omitempty"`
	ReceivingFacilityID *string    `json:"receiving_facility_id,omitempty"`
	ReferringFacilityID *string    `json:"referring_facility_id,omitempty"`
	PriorityID          *string    `json:"priority_id,omitempty"`
	DateFrom            *time.Time `json:"date_from,omitempty"`
	DateTo              *time.Time `json:"date_to,omitempty"`
	// AssignedToUserID, when set, restricts results to incidents that have a
	// dispatch assignment where the user is the driver or lead medic. It is
	// derived server-side from the caller's roles and is not client-supplied.
	AssignedToUserID *string               `json:"assigned_to_user_id,omitempty"`
	Pagination       platformdb.Pagination `json:"pagination"`
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
