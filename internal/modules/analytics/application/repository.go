package application

import (
	"context"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

type Repository interface {
	GetSummary(ctx context.Context, filters analyticsdomain.Filters) (analyticsdomain.Summary, error)
	GetFuelAnalytics(ctx context.Context, filters analyticsdomain.FuelFilters) (analyticsdomain.FuelAnalytics, error)
}
