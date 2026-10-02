package application

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// Export datasets available as CSV registers.
const (
	ExportIncidents  = "incidents"
	ExportDispatches = "dispatches"
	ExportFuel       = "fuel"
	ExportReferrals  = "referrals"
)

var exportDatasets = map[string]bool{
	ExportIncidents: true, ExportDispatches: true, ExportFuel: true, ExportReferrals: true,
}

// resolve validates the query and fixes the reporting window. All-time reports
// start at the first incident in scope.
func (s *Service) resolve(ctx context.Context, q ReportQuery) (analyticsdomain.ReportFilters, error) {
	var start *time.Time
	if strings.TrimSpace(q.DateFrom) == "" {
		var district *string
		if id := strings.TrimSpace(q.DistrictID); id != "" {
			if _, err := uuid.Parse(id); err != nil {
				return analyticsdomain.ReportFilters{}, invalid("district_id must be a UUID")
			}
			district = &id
		}
		var err error
		if start, err = s.repo.DataStart(ctx, district); err != nil {
			return analyticsdomain.ReportFilters{}, err
		}
	}
	return ResolveReportFilters(q, s.now(), start)
}

func (s *Service) meta(f analyticsdomain.ReportFilters) analyticsdomain.ReportMeta {
	return analyticsdomain.ReportMeta{GeneratedAt: s.now().UTC(), Period: PeriodOf(f)}
}

func (s *Service) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

// Overview is the executive summary: headline KPIs against the previous
// period, the trend, distributions and generated insights.
func (s *Service) Overview(ctx context.Context, q ReportQuery) (analyticsdomain.Overview, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.Overview{}, err
	}
	out, err := s.repo.Overview(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	out.KPIs = BuildKPIs(out.Facts, out.Trend, f.AllTime)
	out.Insights = BuildInsights(out.Facts, f.AllTime)
	return out, nil
}

// Demand reports when incidents happen and projects the next fortnight.
func (s *Service) Demand(ctx context.Context, q ReportQuery) (analyticsdomain.Demand, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.Demand{}, err
	}
	out, err := s.repo.Demand(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	DeriveDemand(&out, f, s.now())
	return out, nil
}

func (s *Service) Response(ctx context.Context, q ReportQuery) (analyticsdomain.ResponsePerformance, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.ResponsePerformance{}, err
	}
	out, err := s.repo.Response(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	return out, nil
}

func (s *Service) Referrals(ctx context.Context, q ReportQuery) (analyticsdomain.ReferralNetwork, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.ReferralNetwork{}, err
	}
	out, err := s.repo.Referrals(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	return out, nil
}

func (s *Service) Geo(ctx context.Context, q ReportQuery) (analyticsdomain.Geo, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.Geo{}, err
	}
	out, err := s.repo.Geo(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	return out, nil
}

func (s *Service) Clinical(ctx context.Context, q ReportQuery) (analyticsdomain.Clinical, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.Clinical{}, err
	}
	out, err := s.repo.Clinical(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	return out, nil
}

func (s *Service) Fleet(ctx context.Context, q ReportQuery) (analyticsdomain.FleetPerformance, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.FleetPerformance{}, err
	}
	out, err := s.repo.Fleet(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	return out, nil
}

func (s *Service) DataQuality(ctx context.Context, q ReportQuery) (analyticsdomain.DataQuality, error) {
	f, err := s.resolve(ctx, q)
	if err != nil {
		return analyticsdomain.DataQuality{}, err
	}
	out, err := s.repo.DataQuality(ctx, f)
	if err != nil {
		return out, err
	}
	out.ReportMeta = s.meta(f)
	out.Score, out.Grade = ScoreQuality(out.Fields)
	return out, nil
}

// ExportFilename is the suggested download name for a register.
func ExportFilename(dataset string, f analyticsdomain.ReportFilters) string {
	last := f.To.AddDate(0, 0, -1)
	return fmt.Sprintf("ems-%s-register_%s_to_%s.csv", dataset, f.From.Format("2006-01-02"), last.Format("2006-01-02"))
}

// PrepareExport validates the dataset and resolves the window so the handler
// can set headers before streaming.
func (s *Service) PrepareExport(ctx context.Context, dataset string, q ReportQuery) (analyticsdomain.ReportFilters, error) {
	if !exportDatasets[dataset] {
		return analyticsdomain.ReportFilters{}, invalid("unknown export dataset")
	}
	return s.resolve(ctx, q)
}

func (s *Service) Export(ctx context.Context, f analyticsdomain.ReportFilters, dataset string, w io.Writer) error {
	return s.repo.Export(ctx, f, dataset, w)
}
