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

	// Funding sources: spent/remaining are computed from linked fuel logs and
	// top-ups.
	ListFundingSources(ctx context.Context) ([]domain.FundingSource, error)
	GetFundingSource(ctx context.Context, id string) (domain.FundingSource, error)
	CreateFundingSource(ctx context.Context, in domain.FundingSource) (domain.FundingSource, error)
	UpdateFundingSource(ctx context.Context, id string, req UpdateFundingSourceRequest) (domain.FundingSource, error)
	DeleteFundingSource(ctx context.Context, id string) error

	// Top-ups add money to a funding source over time.
	AddFundingTopup(ctx context.Context, in domain.FundingTopup) (domain.FundingTopup, error)
	ListFundingTopups(ctx context.Context, fundingSourceID string) ([]domain.FundingTopup, error)
}
