package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	incidentdomain "dispatch/internal/modules/incidents/domain"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/events"
)

type Service struct {
	repo Repository
	bus  events.Publisher
	log  *zap.Logger
}

func NewService(repo Repository, bus events.Publisher, log *zap.Logger) *Service {
	return &Service{repo: repo, bus: bus, log: log}
}

func (s *Service) CreateIncident(ctx context.Context, req CreateIncidentRequest) (CreateIncidentResponse, error) {
	incidentNumber, err := s.repo.NextIncidentNumber(ctx)
	if err != nil {
		return CreateIncidentResponse{}, err
	}
	status := "NEW"
	verificationStatus := "PENDING"
	now := time.Now()

	var facilityID *string
	if req.FacilityID != nil && strings.TrimSpace(*req.FacilityID) != "" {
		facilityID = req.FacilityID
	}

	inc := incidentdomain.Incident{
		ID:                 uuid.NewString(),
		IncidentNumber:     incidentNumber,
		SourceChannel:      strings.ToUpper(strings.TrimSpace(req.SourceChannel)),
		CallerName:         req.CallerName,
		CallerPhone:        req.CallerPhone,
		PatientName:        req.PatientName,
		PatientPhone:       req.PatientPhone,
		PatientAgeGroup:    req.PatientAgeGroup,
		PatientSex:         strings.ToUpper(strings.TrimSpace(req.PatientSex)),
		IncidentTypeID:     req.IncidentTypeID,
		SeverityLevelID:    req.SeverityLevelID,
		PriorityLevelID:    req.PriorityLevelID,
		Summary:            req.Summary,
		Description:        req.Description,
		DistrictID:         req.DistrictID,
		FacilityID:         facilityID,
		Village:            req.Village,
		Parish:             req.Parish,
		Subcounty:          req.Subcounty,
		Landmark:           req.Landmark,
		Latitude:           req.Latitude,
		Longitude:          req.Longitude,
		VerificationStatus: verificationStatus,
		Status:             status,
		ReportedAt:         now,
		CreatedByUserID:    req.CreatedByUserID,
	}
	created, err := s.repo.CreateIncident(ctx, inc)
	if err != nil {
		return CreateIncidentResponse{}, err
	}
	_ = s.repo.CreateIncidentUpdate(ctx, created.ID, "COMMENT", "", "", "incident created", req.CreatedByUserID)

	resp := CreateIncidentResponse{
		Incident:                   created,
		AutoDispatchEligible:       false,
		DispatchRecommendationHint: buildDispatchRecommendationHint(false, created.PriorityCode),
	}

	if len(req.TriageResponses) > 0 {
		questionnaireCode := strings.ToUpper(strings.TrimSpace(req.QuestionnaireCode))
		if questionnaireCode == "" {
			questionnaireCode = "EMS_PRIMARY_TRIAGE"
		}
		triage, triageErr := s.persistTriage(ctx, created.ID, questionnaireCode, req.TriageResponses, req.TriageNotes, req.CreatedByUserID, "triage persisted on incident creation")
		if triageErr != nil {
			return CreateIncidentResponse{}, triageErr
		}
		updatedIncident, _ := s.repo.GetIncidentByID(ctx, created.ID)
		resp.Incident = updatedIncident
		resp.TriageSession = &triage
		resp.AutoDispatchEligible = triage.AutoDispatchEligible
		resp.DispatchRecommendationHint = buildDispatchRecommendationHint(triage.AutoDispatchEligible, triage.DerivedPriorityCode)
	}

	_ = s.bus.Publish(ctx, "incident.created", events.Event{
		ID:          uuid.NewString(),
		Topic:       "incident.created",
		AggregateID: created.ID,
		Type:        "incident.created",
		OccurredAt:  now,
		Payload: map[string]any{
			"incident_id":     created.ID,
			"incident_number": created.IncidentNumber,
			"source_channel":  created.SourceChannel,
			"priority_code":   resp.Incident.PriorityCode,
			"auto_dispatch":   resp.AutoDispatchEligible,
		},
	})

	return resp, nil
}

func (s *Service) persistTriage(ctx context.Context, incidentID, questionnaireCode string, inputs []TriageResponseInput, notes string, actorUserID *string, auditMessage string) (incidentdomain.PersistedTriageSession, error) {
	questionnaireID, err := s.repo.ResolveQuestionnaireIDByCode(ctx, questionnaireCode)
	if err != nil {
		return incidentdomain.PersistedTriageSession{}, err
	}
	defs, err := s.repo.GetQuestionDefinitions(ctx, questionnaireCode)
	if err != nil {
		return incidentdomain.PersistedTriageSession{}, err
	}

	session := incidentdomain.PersistedTriageSession{
		ID:                uuid.NewString(),
		IncidentID:        incidentID,
		QuestionnaireID:   questionnaireID,
		QuestionnaireCode: questionnaireCode,
		TriageMode:        "PRIMARY",
		Notes:             notes,
		TriagedByUserID:   actorUserID,
		TriagedAt:         time.Now().UTC(),
	}

	responses := make([]incidentdomain.PersistedTriageResponse, 0, len(inputs))
	booleanTrueCount := 0
	totalScore := 0

	for _, in := range inputs {
		code := strings.ToUpper(strings.TrimSpace(in.QuestionCode))
		raw := strings.TrimSpace(in.ResponseValue)
		def, ok := defs[code]
		if !ok {
			continue
		}
		resp := incidentdomain.PersistedTriageResponse{
			QuestionID:   def.QuestionID,
			QuestionCode: code,
			ResponseType: def.ResponseType,
		}

		switch def.ResponseType {
		case "BOOLEAN":
			v := strings.EqualFold(raw, "true") || strings.EqualFold(raw, "yes")
			resp.ResponseValueBool = &v
			txt := strings.ToLower(raw)
			resp.ResponseValueText = &txt
			if v {
				booleanTrueCount++
			}
			if def.TrueScore != nil && v {
				resp.ScoreAwarded = *def.TrueScore
			}
			if def.FalseScore != nil && !v {
				resp.ScoreAwarded = *def.FalseScore
			}
		case "INTEGER":
			n, err := strconv.Atoi(raw)
			if err == nil {
				resp.ResponseValueInt = &n
				txt := raw
				resp.ResponseValueText = &txt
				switch {
				case n >= 5:
					resp.ScoreAwarded = 90
				case n >= 3:
					resp.ScoreAwarded = 50
				case n >= 1:
					resp.ScoreAwarded = 10
				}
			}
		default:
			txt := raw
			resp.ResponseValueText = &txt
		}

		totalScore += resp.ScoreAwarded
		responses = append(responses, resp)
	}

	session.BooleanTrueCount = booleanTrueCount
	session.TotalScore = totalScore
	session.AutoDispatchEligible = booleanTrueCount >= 3

	priorityCode := "GREEN"
	switch {
	case session.AutoDispatchEligible:
		priorityCode = "RED"
	case totalScore >= 90:
		priorityCode = "RED"
	case totalScore >= 40:
		priorityCode = "ORANGE"
	default:
		priorityCode = "GREEN"
	}
	priorityID, err := s.repo.ResolvePriorityLevelIDByCode(ctx, priorityCode)
	if err != nil {
		return incidentdomain.PersistedTriageSession{}, err
	}
	session.DerivedPriorityLevelID = priorityID
	session.DerivedPriorityCode = priorityCode
	session.Responses = responses

	created, err := s.repo.CreatePersistedTriageSession(ctx, session)
	if err != nil {
		return incidentdomain.PersistedTriageSession{}, err
	}
	if err := s.repo.SetIncidentPriorityByCode(ctx, incidentID, priorityCode); err != nil {
		return incidentdomain.PersistedTriageSession{}, err
	}
	_ = s.repo.SetIncidentTriageSummary(ctx, incidentID, actorUserID)
	if created.AutoDispatchEligible || priorityCode == "RED" || priorityCode == "ORANGE" {
		_, _ = s.repo.UpdateIncidentStatus(ctx, incidentID, "AWAITING_ASSIGNMENT")
	}
	_ = s.repo.CreateIncidentUpdate(ctx, incidentID, "TRIAGE", "", priorityCode, fmt.Sprintf("%s: score=%d, boolean_true_count=%d", auditMessage, totalScore, booleanTrueCount), actorUserID)
	return created, nil
}

func (s *Service) GetIncidentByID(ctx context.Context, id string) (incidentdomain.Incident, error) {
	return s.repo.GetIncidentByID(ctx, id)
}

func (s *Service) ListIncidents(ctx context.Context, params ListIncidentsParams) (platformdb.PageResult[incidentdomain.Incident], error) {
	items, total, err := s.repo.ListIncidents(ctx, params)
	if err != nil {
		return platformdb.PageResult[incidentdomain.Incident]{}, err
	}
	return platformdb.PageResult[incidentdomain.Incident]{Items: items, Meta: platformdb.NewPageMeta(params.Pagination, total)}, nil
}

func (s *Service) UpdateIncidentStatus(ctx context.Context, id string, req UpdateIncidentStatusRequest, actorUserID *string) (incidentdomain.Incident, error) {
	updated, err := s.repo.UpdateIncidentStatus(ctx, id, strings.ToUpper(strings.TrimSpace(req.Status)))
	if err != nil {
		return incidentdomain.Incident{}, err
	}
	_ = s.repo.CreateIncidentUpdate(ctx, id, "STATUS_CHANGE", "", updated.Status, req.Notes, actorUserID)
	return updated, nil
}

func (s *Service) UpdateIncident(ctx context.Context, id string, req UpdateIncidentRequest, actorUserID *string) (UpdateIncidentResponse, error) {
	current, err := s.repo.GetIncidentByID(ctx, id)
	if err != nil {
		return UpdateIncidentResponse{}, err
	}
	updated, err := s.repo.UpdateIncident(ctx, id, req)
	if err != nil {
		return UpdateIncidentResponse{}, err
	}
	s.recordIncidentAttributeChanges(ctx, id, current, updated, req.Notes, actorUserID)

	resp := UpdateIncidentResponse{
		Incident:                   updated,
		AutoDispatchEligible:       false,
		DispatchRecommendationHint: buildDispatchRecommendationHint(false, updated.PriorityCode),
	}

	if len(req.TriageResponses) > 0 {
		questionnaireCode := strings.ToUpper(strings.TrimSpace(req.QuestionnaireCode))
		if questionnaireCode == "" {
			questionnaireCode = "EMS_PRIMARY_TRIAGE"
		}
		triage, triageErr := s.persistTriage(ctx, id, questionnaireCode, req.TriageResponses, req.TriageNotes, actorUserID, "triage persisted during incident update")
		if triageErr != nil {
			return UpdateIncidentResponse{}, triageErr
		}
		refreshed, refreshErr := s.repo.GetIncidentByID(ctx, id)
		if refreshErr != nil {
			return UpdateIncidentResponse{}, refreshErr
		}
		resp.Incident = refreshed
		resp.TriageSession = &triage
		resp.AutoDispatchEligible = triage.AutoDispatchEligible
		resp.DispatchRecommendationHint = buildDispatchRecommendationHint(triage.AutoDispatchEligible, triage.DerivedPriorityCode)
	}

	return resp, nil
}

func (s *Service) recordIncidentAttributeChanges(ctx context.Context, incidentID string, before, after incidentdomain.Incident, notes string, actorUserID *string) {
	type fieldChange struct {
		field      string
		updateType string
		oldValue   string
		newValue   string
	}

	changes := []fieldChange{
		{field: "source_channel", updateType: "COMMENT", oldValue: before.SourceChannel, newValue: after.SourceChannel},
		{field: "caller_name", updateType: "COMMENT", oldValue: before.CallerName, newValue: after.CallerName},
		{field: "caller_phone", updateType: "COMMENT", oldValue: before.CallerPhone, newValue: after.CallerPhone},
		{field: "patient_name", updateType: "COMMENT", oldValue: before.PatientName, newValue: after.PatientName},
		{field: "patient_phone", updateType: "COMMENT", oldValue: before.PatientPhone, newValue: after.PatientPhone},
		{field: "patient_age_group", updateType: "COMMENT", oldValue: before.PatientAgeGroup, newValue: after.PatientAgeGroup},
		{field: "patient_sex", updateType: "COMMENT", oldValue: before.PatientSex, newValue: after.PatientSex},
		{field: "incident_type_id", updateType: "COMMENT", oldValue: before.IncidentTypeID, newValue: after.IncidentTypeID},
		{field: "severity_level_id", updateType: "COMMENT", oldValue: stringValue(before.SeverityLevelID), newValue: stringValue(after.SeverityLevelID)},
		{field: "priority_level_id", updateType: "COMMENT", oldValue: stringValue(before.PriorityLevelID), newValue: stringValue(after.PriorityLevelID)},
		{field: "summary", updateType: "COMMENT", oldValue: before.Summary, newValue: after.Summary},
		{field: "description", updateType: "COMMENT", oldValue: before.Description, newValue: after.Description},
		{field: "district_id", updateType: "LOCATION_UPDATE", oldValue: stringValue(before.DistrictID), newValue: stringValue(after.DistrictID)},
		{field: "facility_id", updateType: "LOCATION_UPDATE", oldValue: stringValue(before.FacilityID), newValue: stringValue(after.FacilityID)},
		{field: "village", updateType: "LOCATION_UPDATE", oldValue: before.Village, newValue: after.Village},
		{field: "parish", updateType: "LOCATION_UPDATE", oldValue: before.Parish, newValue: after.Parish},
		{field: "subcounty", updateType: "LOCATION_UPDATE", oldValue: before.Subcounty, newValue: after.Subcounty},
		{field: "landmark", updateType: "LOCATION_UPDATE", oldValue: before.Landmark, newValue: after.Landmark},
		{field: "latitude", updateType: "LOCATION_UPDATE", oldValue: floatValue(before.Latitude), newValue: floatValue(after.Latitude)},
		{field: "longitude", updateType: "LOCATION_UPDATE", oldValue: floatValue(before.Longitude), newValue: floatValue(after.Longitude)},
		{field: "verification_status", updateType: "VERIFICATION", oldValue: before.VerificationStatus, newValue: after.VerificationStatus},
		{field: "status", updateType: "STATUS_CHANGE", oldValue: before.Status, newValue: after.Status},
		{field: "reported_at", updateType: "COMMENT", oldValue: timeValue(before.ReportedAt), newValue: timePointerValue(after.ReportedAt)},
	}

	changed := false
	for _, change := range changes {
		if change.oldValue == change.newValue {
			continue
		}
		changed = true
		_ = s.repo.CreateIncidentUpdate(
			ctx,
			incidentID,
			change.updateType,
			change.oldValue,
			change.newValue,
			buildIncidentChangeNote(change.field, notes),
			actorUserID,
		)
	}

	if !changed && strings.TrimSpace(notes) != "" {
		_ = s.repo.CreateIncidentUpdate(ctx, incidentID, "COMMENT", "", "", strings.TrimSpace(notes), actorUserID)
	}
}

func buildIncidentChangeNote(field, notes string) string {
	base := strings.ReplaceAll(field, "_", " ") + " updated"
	if strings.TrimSpace(notes) == "" {
		return base
	}
	return base + ": " + strings.TrimSpace(notes)
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func floatValue(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}

func timeValue(v time.Time) string {
	if v.IsZero() {
		return ""
	}
	return v.UTC().Format(time.RFC3339)
}

func timePointerValue(v time.Time) string {
	return timeValue(v)
}

func buildDispatchRecommendationHint(autoDispatch bool, priorityCode string) string {
	if autoDispatch || priorityCode == "RED" || priorityCode == "ORANGE" {
		return "eligible for dispatch recommendations"
	}
	return "manual review"
}
