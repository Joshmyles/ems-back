package application

import (
	"context"

	"dispatch/internal/modules/fuel/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository interface {
	// List returns paginated fuel logs. When driverUserID is non-nil the
	// results are restricted to fuel logs whose ambulance currently has that
	// user as the active driver in ambulance_crew_assignments.
	List(ctx context.Context, p platformdb.Pagination, driverUserID *string) ([]domain.FuelLog, int64, error)
	// GetByID returns a single fuel log. When driverUserID is non-nil the
	// lookup is constrained to fuel logs on an ambulance the user is the
	// active driver of.
	GetByID(ctx context.Context, id string, driverUserID *string) (domain.FuelLog, error)
	Create(ctx context.Context, in domain.FuelLog) (domain.FuelLog, error)
	Update(ctx context.Context, id string, req UpdateFuelLogRequest) (domain.FuelLog, error)
	Delete(ctx context.Context, id string) error

	GetPublicByToken(ctx context.Context, token string) (domain.FuelLogPublicView, error)
	ConfirmDispense(ctx context.Context, token string, req ConfirmFuelDispenseRequest) (int64, error)
}
