package application

import (
	"context"

	"dispatch/internal/modules/responders/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository interface {
	// ListResponders returns paginated responders aggregated from ambulances,
	// their active crew/driver, station and current dispatch load.
	ListResponders(ctx context.Context, p platformdb.Pagination) ([]domain.Responder, int64, error)
	// GetByID returns a single responder (ambulance) by ambulance id.
	GetByID(ctx context.Context, id string) (domain.Responder, error)
}
