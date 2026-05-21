package application

import (
	"context"

	"dispatch/internal/modules/facilities/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository interface {
	ListFacilities(ctx context.Context, p platformdb.Pagination) ([]domain.Facility, int64, error)
	GetByUID(ctx context.Context, uid string) (domain.Facility, error)
	Create(ctx context.Context, in domain.Facility) (domain.Facility, error)
	Update(ctx context.Context, uid string, req UpdateFacilityRequest) (domain.Facility, error)
	Delete(ctx context.Context, uid string) error
	SetFocalPerson(ctx context.Context, facilityUID string, userID string) (domain.Facility, error)
	ClearFocalPerson(ctx context.Context, facilityUID string) (domain.Facility, error)
}
