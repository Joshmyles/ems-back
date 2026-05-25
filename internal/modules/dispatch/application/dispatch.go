package application

import (
	"context"
	"dispatch/internal/modules/dispatch/application/dto"
	dispatchdomain "dispatch/internal/modules/dispatch/domain"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/events"

	"github.com/google/uuid"
)

var (
	ErrDispatchNotEligible = errors.New("incident does not qualify for automatic dispatch")
)

type Service struct {
	repo          Repository
	bus           events.Publisher
	notifications NotificationCreator
}

func NewService(repo Repository, bus events.Publisher, notifications NotificationCreator) *Service {
	return &Service{repo: repo, bus: bus, notifications: notifications}
}

func (s *Service) CreateAssignment(ctx context.Context, req dto.CreateDispatchAssignmentRequest) (dispatchdomain.DispatchAssignment, error) {
	mode := strings.ToUpper(strings.TrimSpace(req.AssignmentMode))
	if mode == "" {
		mode = "MANUAL"
	}
	now := time.Now().UTC()
	in := dispatchdomain.DispatchAssignment{
		ID:               uuid.NewString(),
		IncidentID:       req.IncidentID,
		AmbulanceID:      req.AmbulanceID,
		AssignedByUserID: req.AssignedByUserID,
		DriverUserID:     req.DriverUserID,
		LeadMedicUserID:  req.LeadMedicUserID,
		AssignmentMode:   mode,
		RankingScore:     req.RankingScore,
		ETAMinutes:       req.ETAMinutes,
		Status:           "ASSIGNED",
		AssignedAt:       &now,
	}
	created, err := s.repo.CreateAssignment(ctx, in)
	if err != nil {
		return dispatchdomain.DispatchAssignment{}, err
	}
	_ = s.repo.UpdateIncidentStatus(ctx, req.IncidentID, "ASSIGNED")
	if req.AmbulanceID != nil {
		_ = s.repo.MarkRecommendationSelected(ctx, req.IncidentID, *req.AmbulanceID)
	}
	_ = s.repo.SetUserAvailabilityBusy(ctx, req.IncidentID, created.ID, req.AmbulanceID, req.DriverUserID, req.LeadMedicUserID)
	_ = s.repo.CreateIncidentUpdate(ctx, req.IncidentID, "STATUS_CHANGE", "AWAITING_ASSIGNMENT", "ASSIGNED", "dispatch assignment created", req.AssignedByUserID)
	s.notifyAssignmentRecipients(ctx, created)
	_ = s.bus.Publish(ctx, "dispatch.assigned", events.Event{
		ID:          uuid.NewString(),
		Topic:       "dispatch.assigned",
		AggregateID: created.ID,
		Type:        "dispatch.assigned",
		OccurredAt:  now,
		Payload: map[string]any{
			"dispatch_assignment_id": created.ID,
			"incident_id":            created.IncidentID,
			"ambulance_id":           created.AmbulanceID,
			"driver_user_id":         created.DriverUserID,
			"lead_medic_user_id":     created.LeadMedicUserID,
			"assignment_mode":        created.AssignmentMode,
		},
	})
	return created, nil
}

func (s *Service) notifyAssignmentRecipients(ctx context.Context, assignment dispatchdomain.DispatchAssignment) {
	if s.notifications == nil {
		return
	}

	recipients := uniqueUserIDs(assignment.DriverUserID, assignment.LeadMedicUserID)
	if len(recipients) == 0 {
		return
	}

	title := "Incident assigned"
	body := fmt.Sprintf("You have been assigned to incident %s.", assignment.IncidentID)
	entityType := "DISPATCH_ASSIGNMENT"
	entityID := assignment.ID

	for _, recipientID := range recipients {
		recipientID := recipientID
		_, _ = s.notifications.Create(
			ctx,
			"DISPATCH_ASSIGNMENT",
			"IN_APP",
			&recipientID,
			nil,
			nil,
			&title,
			&entityType,
			body,
			&entityID,
		)
		_, _ = s.notifications.Create(
			ctx,
			"DISPATCH_ASSIGNMENT",
			"PUSH",
			&recipientID,
			nil,
			nil,
			&title,
			&entityType,
			body,
			&entityID,
		)
	}
}

func uniqueUserIDs(values ...*string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == nil {
			continue
		}
		id := strings.TrimSpace(*value)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *Service) UpdateAssignmentStatus(ctx context.Context, id string, req dto.UpdateDispatchStatusRequest, actorUserID *string) (dispatchdomain.DispatchAssignmentResponse, error) {
	status := strings.ToUpper(strings.TrimSpace(req.Status))
	updated, err := s.repo.UpdateAssignmentStatus(ctx, id, status, req.CancellationReason)
	if err != nil {
		return dispatchdomain.DispatchAssignmentResponse{}, err
	}
	incidentStatus := map[string]string{
		"ACCEPTED":            "ENROUTE",
		"DEPARTED":            "ENROUTE",
		"ARRIVED_SCENE":       "AT_SCENE",
		"PATIENT_LOADED":      "TRANSPORTING",
		"ARRIVED_DESTINATION": "TRANSPORTING",
		"COMPLETED":           "COMPLETED",
		"CANCELLED":           "AWAITING_ASSIGNMENT",
	}
	if st, ok := incidentStatus[status]; ok {
		_ = s.repo.UpdateIncidentStatus(ctx, updated.IncidentID, st)
	}
	_ = s.repo.CreateIncidentUpdate(ctx, updated.IncidentID, "STATUS_CHANGE", "", status, "dispatch assignment status updated", actorUserID)
	return updated, nil
}

func (s *Service) GetAssignmentByID(ctx context.Context, id string) (dispatchdomain.DispatchAssignmentResponse, error) {
	return s.repo.GetAssignmentByID(ctx, id)
}

func (s *Service) ListAssignments(ctx context.Context, params dto.ListAssignmentsParams) (platformdb.PageResult[dispatchdomain.DispatchAssignmentResponse], error) {
	items, total, err := s.repo.ListAssignments(ctx, params)
	if err != nil {
		return platformdb.PageResult[dispatchdomain.DispatchAssignmentResponse]{}, err
	}
	return platformdb.PageResult[dispatchdomain.DispatchAssignmentResponse]{Items: items, Meta: platformdb.NewPageMeta(params.Pagination, total)}, nil
}

func (s *Service) ListRecommendations(ctx context.Context, params dto.ListRecommendationsParams) (platformdb.PageResult[dispatchdomain.DispatchRecommendation], error) {
	items, total, err := s.repo.ListRecommendations(ctx, params)
	if err != nil {
		return platformdb.PageResult[dispatchdomain.DispatchRecommendation]{}, err
	}
	return platformdb.PageResult[dispatchdomain.DispatchRecommendation]{Items: items, Meta: platformdb.NewPageMeta(params.Pagination, total)}, nil
}

func (s *Service) PersistTriageSession(ctx context.Context, req dto.PersistTriageRequest, actorUserID *string) (dispatchdomain.PersistedTriageSession, error) {
	questionnaireCode := strings.ToUpper(strings.TrimSpace(req.QuestionnaireCode))
	if questionnaireCode == "" {
		questionnaireCode = "EMS_PRIMARY_TRIAGE"
	}

	questionnaireID, err := s.repo.ResolveQuestionnaireIDByCode(ctx, questionnaireCode)
	if err != nil {
		return dispatchdomain.PersistedTriageSession{}, err
	}

	defs, err := s.repo.GetQuestionDefinitions(ctx, questionnaireCode)
	if err != nil {
		return dispatchdomain.PersistedTriageSession{}, err
	}

	session := dispatchdomain.PersistedTriageSession{
		ID:              uuid.NewString(),
		IncidentID:      req.IncidentID,
		QuestionnaireID: questionnaireID,
		TriageMode:      strings.ToUpper(strings.TrimSpace(req.TriageMode)),
		Notes:           req.Notes,
		TriagedByUserID: actorUserID,
		TriagedAt:       time.Now().UTC(),
	}
	if session.TriageMode == "" {
		session.TriageMode = "PRIMARY"
	}

	responses := make([]dispatchdomain.PersistedTriageResponse, 0, len(req.Responses))
	booleanTrueCount := 0
	totalScore := 0

	for _, in := range req.Responses {
		code := strings.ToUpper(strings.TrimSpace(in.QuestionCode))
		raw := strings.TrimSpace(in.ResponseValue)

		def, ok := defs[code]
		if !ok {
			continue
		}

		resp := dispatchdomain.PersistedTriageResponse{
			QuestionID:   def.QuestionID,
			QuestionCode: code,
			ResponseType: def.ResponseType,
			ScoreAwarded: 0,
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
				if code == "HOW_MANY_PEOPLE_INJURED" {
					switch {
					case n >= 5:
						resp.ScoreAwarded = 90
					case n >= 3:
						resp.ScoreAwarded = 50
					case n >= 1:
						resp.ScoreAwarded = 10
					}
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
		return dispatchdomain.PersistedTriageSession{}, err
	}
	session.DerivedPriorityLevelID = priorityID
	session.DerivedPriorityCode = priorityCode
	session.Responses = responses

	created, err := s.repo.CreatePersistedTriageSession(ctx, session)
	if err != nil {
		return dispatchdomain.PersistedTriageSession{}, err
	}

	if err := s.repo.SetIncidentPriorityByCode(ctx, req.IncidentID, priorityCode); err != nil {
		return dispatchdomain.PersistedTriageSession{}, err
	}
	_ = s.repo.SetIncidentTriageSummary(ctx, req.IncidentID, actorUserID)
	_ = s.repo.CreateIncidentUpdate(ctx, req.IncidentID, "TRIAGE", "", priorityCode, fmt.Sprintf("triage persisted: score=%d, boolean_true_count=%d", totalScore, booleanTrueCount), actorUserID)

	if created.AutoDispatchEligible {
		_ = s.repo.UpdateIncidentStatus(ctx, req.IncidentID, "AWAITING_ASSIGNMENT")
	}

	return created, nil
}

func (s *Service) EvaluateAutomaticDispatch(ctx context.Context, req dto.EvaluateDispatchRequest, actorUserID *string) (map[string]any, error) {
	persisted, err := s.PersistTriageSession(ctx, dto.PersistTriageRequest{
		IncidentID:        req.IncidentID,
		QuestionnaireCode: "EMS_PRIMARY_TRIAGE",
		TriageMode:        "PRIMARY",
		Responses:         req.Responses,
	}, actorUserID)
	if err != nil {
		return nil, err
	}

	shouldDispatch := persisted.AutoDispatchEligible || persisted.DerivedPriorityCode == "RED" || persisted.DerivedPriorityCode == "ORANGE"

	return map[string]any{
		"incident_id":           req.IncidentID,
		"triage_session_id":     persisted.ID,
		"true_boolean_count":    persisted.BooleanTrueCount,
		"priority_code":         persisted.DerivedPriorityCode,
		"auto_dispatch":         persisted.AutoDispatchEligible,
		"eligible_for_dispatch": shouldDispatch,
	}, nil
}

func (s *Service) GenerateRecommendations(ctx context.Context, req dto.GenerateRecommendationsRequest, actorUserID *string) ([]dispatchdomain.DispatchRecommendation, error) {
	incident, err := s.repo.GetIncidentDispatchContext(ctx, req.IncidentID)
	if err != nil {
		return nil, err
	}

	latestTriage, err := s.repo.GetLatestTriageSession(ctx, req.IncidentID)
	if err == nil {
		incident.PriorityCode = latestTriage.DerivedPriorityCode
	}

	priorityCode := strings.ToUpper(strings.TrimSpace(incident.PriorityCode))
	if req.Auto && priorityCode != "RED" && priorityCode != "ORANGE" {
		return nil, ErrDispatchNotEligible
	}

	candidates, err := s.repo.FindDispatchCandidates(ctx, req.IncidentID, 10)
	if err != nil {
		return nil, err
	}

	recs := make([]dispatchdomain.DispatchRecommendation, 0, len(candidates))
	for _, c := range candidates {
		score := 0.0
		rules := make([]string, 0)

		if c.Dispatchable {
			score += 60
			rules = append(rules, "dispatchable crew")
		}
		if strings.ToUpper(c.Availability) == "AVAILABLE" || strings.ToUpper(c.Availability) == "STANDBY" {
			score += 25
			rules = append(rules, "available status")
		}
		if c.ETAMinutes != nil {
			score += math.Max(0, 20-float64(*c.ETAMinutes))
			rules = append(rules, fmt.Sprintf("eta %d mins", *c.ETAMinutes))
		}
		if incident.DistrictID != nil && c.DistrictID != nil && *incident.DistrictID == *c.DistrictID {
			score += 10
			rules = append(rules, "same district")
		}

		recs = append(recs, dispatchdomain.DispatchRecommendation{
			ID:           uuid.NewString(),
			IncidentID:   req.IncidentID,
			AmbulanceID:  c.AmbulanceID,
			DriverUserID: c.DriverUserID,
			Score:        score,
			ETAMinutes:   c.ETAMinutes,
			RuleSummary:  strings.Join(rules, "; "),
			GeneratedAt:  time.Now().UTC(),
			Selected:     false,
		})
	}

	sort.SliceStable(recs, func(i, j int) bool { return recs[i].Score > recs[j].Score })

	if err := s.repo.ReplaceRecommendations(ctx, req.IncidentID, recs); err != nil {
		return nil, err
	}
	_ = s.repo.CreateIncidentUpdate(ctx, req.IncidentID, "TRIAGE", "", "", fmt.Sprintf("%d dispatch recommendation(s) generated", len(recs)), actorUserID)

	return recs, nil
}
