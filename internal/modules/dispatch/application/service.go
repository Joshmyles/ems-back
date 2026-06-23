package application

import (
	"context"

	"dispatch/internal/modules/dispatch/application/dto"
	dispatchdomain "dispatch/internal/modules/dispatch/domain"
	notifdomain "dispatch/internal/modules/notifications/domain"
)

type Repository interface {
	GetIncidentDispatchContext(ctx context.Context, incidentID string) (dispatchdomain.IncidentDispatchContext, error)

	ResolveQuestionnaireIDByCode(ctx context.Context, questionnaireCode string) (string, error)
	GetQuestionDefinitions(ctx context.Context, questionnaireCode string) (map[string]dispatchdomain.QuestionDefinition, error)
	ResolvePriorityLevelIDByCode(ctx context.Context, priorityCode string) (*string, error)
	CreatePersistedTriageSession(ctx context.Context, session dispatchdomain.PersistedTriageSession) (dispatchdomain.PersistedTriageSession, error)
	GetLatestTriageSession(ctx context.Context, incidentID string) (dispatchdomain.PersistedTriageSession, error)

	ResolvePriorityCodeByIncident(ctx context.Context, incidentID string) (string, error)
	SetIncidentPriorityByCode(ctx context.Context, incidentID, priorityCode string) error
	SetIncidentTriageSummary(ctx context.Context, incidentID string, triagedByUserID *string) error
	UpdateIncidentStatus(ctx context.Context, incidentID, status string) error
	CreateIncidentUpdate(ctx context.Context, incidentID, updateType, oldValue, newValue, notes string, actorUserID *string) error

	FindDispatchCandidates(ctx context.Context, incidentID string, limit int) ([]dispatchdomain.AmbulanceCandidate, error)
	ReplaceRecommendations(ctx context.Context, incidentID string, recs []dispatchdomain.DispatchRecommendation) error
	ListRecommendations(ctx context.Context, params dto.ListRecommendationsParams) ([]dispatchdomain.DispatchRecommendation, int64, error)

	CreateAssignment(ctx context.Context, in dispatchdomain.DispatchAssignment) (dispatchdomain.DispatchAssignment, error)
	GetAssignmentByID(ctx context.Context, id string) (dispatchdomain.DispatchAssignmentResponse, error)
	UpdateAssignmentStatus(ctx context.Context, id, status, cancellationReason string) (dispatchdomain.DispatchAssignmentResponse, error)
	ListAssignments(ctx context.Context, params dto.ListAssignmentsParams) ([]dispatchdomain.DispatchAssignmentResponse, int64, error)
	MarkRecommendationSelected(ctx context.Context, incidentID, ambulanceID string) error
	SetUserAvailabilityBusy(ctx context.Context, incidentID, assignmentID string, ambulanceID, driverUserID, medicUserID *string) error
}

type NotificationCreator interface {
	Create(ctx context.Context,
		typ, channel string,
		recipientUserID, recipientPhone, recipientEmail, title, linkedEntityType *string,
		body string,
		linkedEntityID *string,
	) (notifdomain.Notification, error)
}
