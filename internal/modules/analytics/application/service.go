package application

import (
	"context"
	"time"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetSummary parses the query filters, fetches the aggregated data and stamps
// the derived/meta fields the repository does not populate.
func (s *Service) GetSummary(ctx context.Context, q SummaryQuery) (analyticsdomain.Summary, error) {
	filters := analyticsdomain.Filters{}

	if q.DateFrom != "" {
		if t, err := time.Parse("2006-01-02", q.DateFrom); err == nil {
			filters.DateFrom = &t
		}
	}
	if q.DateTo != "" {
		if t, err := time.Parse("2006-01-02", q.DateTo); err == nil {
			end := t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			filters.DateTo = &end
		}
	}
	if q.DistrictID != "" {
		filters.DistrictID = &q.DistrictID
	}

	out, err := s.repo.GetSummary(ctx, filters)
	if err != nil {
		return analyticsdomain.Summary{}, err
	}

	out.GeneratedAt = time.Now().UTC()
	out.Filters = map[string]any{
		"date_from":   filters.DateFrom,
		"date_to":     filters.DateTo,
		"district_id": filters.DistrictID,
	}

	// Every incident represents one patient case in this system.
	out.PatientReport.TotalPatients = out.Totals.Incidents

	var outcomesReported int64
	for _, o := range out.PatientReport.Outcomes {
		outcomesReported += o.Count
	}
	out.PatientReport.OutcomesReported = outcomesReported

	return out, nil
}
