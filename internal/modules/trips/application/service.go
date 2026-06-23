package application

import (
	"context"
	"time"

	"dispatch/internal/modules/trips/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	repo Repository
	log  *zap.Logger
}

func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

func (s *Service) List(ctx context.Context, p platformdb.Pagination) (platformdb.PageResult[domain.Trip], error) {
	items, total, err := s.repo.ListTrips(ctx, p)
	if err != nil {
		return platformdb.PageResult[domain.Trip]{}, err
	}
	return platformdb.PageResult[domain.Trip]{Items: items, Meta: platformdb.NewPageMeta(p, total)}, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Trip, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, req CreateTripRequest) (domain.Trip, error) {
	now := time.Now().UTC()
	startedAt := parseOptionalTime(req.StartedAt)
	if startedAt == nil {
		startedAt = &now
	}
	endedAt := parseOptionalTime(req.EndedAt)
	in := domain.Trip{
		ID:                    uuid.NewString(),
		DispatchAssignmentID:  req.DispatchAssignmentID,
		IncidentID:            req.IncidentID,
		AmbulanceID:           req.AmbulanceID,
		OriginLat:             req.OriginLat,
		OriginLon:             req.OriginLon,
		SceneLat:              req.SceneLat,
		SceneLon:              req.SceneLon,
		DestinationFacilityID: req.DestinationFacilityID,
		DestinationLat:        req.DestinationLat,
		DestinationLon:        req.DestinationLon,
		OdometerStart:         req.OdometerStart,
		OdometerEnd:           req.OdometerEnd,
		StartedAt:             startedAt,
		EndedAt:               endedAt,
		Outcome:               req.Outcome,
		Notes:                 req.Notes,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	return s.repo.CreateTrip(ctx, in)
}

func parseOptionalTime(raw *string) *time.Time {
	if raw == nil || *raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, *raw)
	if err != nil {
		return nil
	}
	return &parsed
}

func (s *Service) Update(ctx context.Context, id string, req UpdateTripRequest) (domain.Trip, error) {
	return s.repo.UpdateTrip(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.DeleteTrip(ctx, id)
}

func (s *Service) ListEvents(ctx context.Context, tripID string, p platformdb.Pagination) (platformdb.PageResult[domain.TripEvent], error) {
	items, total, err := s.repo.ListTripEvents(ctx, tripID, p)
	if err != nil {
		return platformdb.PageResult[domain.TripEvent]{}, err
	}
	return platformdb.PageResult[domain.TripEvent]{Items: items, Meta: platformdb.NewPageMeta(p, total)}, nil
}

func (s *Service) CreateEvent(ctx context.Context, tripID string, req CreateTripEventRequest) (domain.TripEvent, error) {
	var t time.Time
	if req.EventTime != nil && *req.EventTime != "" {
		parsed, err := time.Parse(time.RFC3339, *req.EventTime)
		if err == nil {
			t = parsed
		}
	}
	if t.IsZero() {
		t = time.Now().UTC()
	}
	in := domain.TripEvent{
		ID:          uuid.NewString(),
		TripID:      tripID,
		EventType:   req.EventType,
		EventTime:   t,
		Latitude:    req.Latitude,
		Longitude:   req.Longitude,
		ActorUserID: req.ActorUserID,
		Notes:       req.Notes,
	}
	return s.repo.CreateTripEvent(ctx, tripID, in)
}
