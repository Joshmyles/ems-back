package application

import (
	"context"

	"dispatch/internal/modules/trips/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository interface {
	ListTrips(ctx context.Context, p platformdb.Pagination) ([]domain.Trip, int64, error)
	GetByID(ctx context.Context, id string) (domain.Trip, error)
	CreateTrip(ctx context.Context, in domain.Trip) (domain.Trip, error)
	UpdateTrip(ctx context.Context, id string, req UpdateTripRequest) (domain.Trip, error)
	DeleteTrip(ctx context.Context, id string) error
	SummarizeTrips(ctx context.Context) (domain.TripSummary, error)

	ListTripEvents(ctx context.Context, tripID string, p platformdb.Pagination) ([]domain.TripEvent, int64, error)
	CreateTripEvent(ctx context.Context, tripID string, in domain.TripEvent) (domain.TripEvent, error)
}
