package application

import (
	"context"
	"io"
	"time"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

type Repository interface {
	GetSummary(ctx context.Context, filters analyticsdomain.Filters) (analyticsdomain.Summary, error)
	GetFuelAnalytics(ctx context.Context, filters analyticsdomain.FuelFilters) (analyticsdomain.FuelAnalytics, error)

	// DataStart is the first incident's report time, used as the start of
	// all-time reports.
	DataStart(ctx context.Context, districtID *string) (*time.Time, error)
	Overview(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Overview, error)
	Demand(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Demand, error)
	Response(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.ResponsePerformance, error)
	Referrals(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.ReferralNetwork, error)
	Geo(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Geo, error)
	Clinical(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.Clinical, error)
	Fleet(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.FleetPerformance, error)
	DataQuality(ctx context.Context, f analyticsdomain.ReportFilters) (analyticsdomain.DataQuality, error)
	Export(ctx context.Context, f analyticsdomain.ReportFilters, dataset string, w io.Writer) error
}
