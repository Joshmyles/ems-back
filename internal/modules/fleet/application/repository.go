package application

import (
	"context"

	"dispatch/internal/modules/fleet/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository interface {
	// ListAmbulances returns paginated ambulances. When driverUserID is non-nil
	// the result is restricted to ambulances where the user is the active driver
	// in ambulance_crew_assignments.
	ListAmbulances(ctx context.Context, p platformdb.Pagination, driverUserID *string) ([]domain.Ambulance, int64, error)
	// GetByID returns a single ambulance. When driverUserID is non-nil the
	// lookup is constrained so a driver cannot read ambulances they are not on.
	GetByID(ctx context.Context, id string, driverUserID *string) (domain.Ambulance, error)
	Create(ctx context.Context, in domain.Ambulance) (domain.Ambulance, error)
	Update(ctx context.Context, id string, req UpdateAmbulanceRequest) (domain.Ambulance, error)
	Delete(ctx context.Context, id string) error
	AssignDriver(ctx context.Context, ambulanceID, driverUserID string) error
	UnassignDriver(ctx context.Context, ambulanceID string) error
}
