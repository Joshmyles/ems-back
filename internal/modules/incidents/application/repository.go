package application

import (
	"context"

	incidentdomain "dispatch/internal/modules/incidents/domain"
)

type QuestionDefinition struct {
	QuestionID   string
	ResponseType string
	TrueScore    *int
	FalseScore   *int
}

type Repository interface {
	CreateIncident(ctx context.Context, in incidentdomain.Incident) (incidentdomain.Incident, error)
	EnsureUnclassifiedIncidentType(ctx context.Context, id string) error
	GetIncidentByID(ctx context.Context, id string) (incidentdomain.Incident, error)
	IsUserAssignedToIncident(ctx context.Context, incidentID, userID string) (bool, error)
	ListIncidents(ctx context.Context, params ListIncidentsParams) ([]incidentdomain.Incident, int64, error)
	UpdateIncident(ctx context.Context, id string, req UpdateIncidentRequest) (incidentdomain.Incident, error)
	UpdateIncidentStatus(ctx context.Context, id, status string) (incidentdomain.Incident, error)
	CreateIncidentUpdate(ctx context.Context, incidentID, updateType, oldValue, newValue, notes string, actorUserID *string) error
	NextIncidentNumber(ctx context.Context) (string, error)
	ResolvePriorityLevelIDByCode(ctx context.Context, code string) (*string, error)
	SetIncidentPriorityByCode(ctx context.Context, incidentID, code string) error
	SetIncidentTriageSummary(ctx context.Context, incidentID string, triagedByUserID *string) error
	ResolveQuestionnaireIDByCode(ctx context.Context, questionnaireCode string) (string, error)
	GetQuestionDefinitions(ctx context.Context, questionnaireCode string) (map[string]QuestionDefinition, error)
	CreatePersistedTriageSession(ctx context.Context, session incidentdomain.PersistedTriageSession) (incidentdomain.PersistedTriageSession, error)
}
