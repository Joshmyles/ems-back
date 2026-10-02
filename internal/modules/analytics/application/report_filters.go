package application

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// ReportTimezone is the zone every report buckets by. Uganda observes no DST,
// so a fixed +03:00 offset is exact and avoids depending on tzdata in the image.
const ReportTimezone = "Africa/Kampala"

var ReportLocation = time.FixedZone("EAT", 3*60*60)

// ErrInvalidReportQuery marks a client error in the report filters.
var ErrInvalidReportQuery = errors.New("invalid report query")

// ReportQuery is the shared query string accepted by every report endpoint.
type ReportQuery struct {
	DateFrom    string `form:"date_from"`
	DateTo      string `form:"date_to"`
	DistrictID  string `form:"district_id"`
	Granularity string `form:"granularity"`
}

func invalid(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidReportQuery, msg)
}

// ResolveReportFilters turns the raw query into a concrete half-open window in
// East Africa Time. date_to is inclusive (the whole day is covered). With no
// date_from the report covers all time, starting at the first recorded
// incident, and period comparison is disabled.
func ResolveReportFilters(q ReportQuery, now time.Time, dataStart *time.Time) (analyticsdomain.ReportFilters, error) {
	f := analyticsdomain.ReportFilters{}
	today := startOfDay(now.In(ReportLocation))

	if q.DateTo != "" {
		t, err := time.ParseInLocation("2006-01-02", q.DateTo, ReportLocation)
		if err != nil {
			return f, invalid("date_to must be YYYY-MM-DD")
		}
		f.To = t.AddDate(0, 0, 1)
	} else {
		f.To = today.AddDate(0, 0, 1)
	}

	if q.DateFrom != "" {
		t, err := time.ParseInLocation("2006-01-02", q.DateFrom, ReportLocation)
		if err != nil {
			return f, invalid("date_from must be YYYY-MM-DD")
		}
		f.From = t
	} else {
		f.AllTime = true
		if dataStart != nil {
			f.From = startOfDay(dataStart.In(ReportLocation))
		} else {
			f.From = f.To.AddDate(0, 0, -30)
		}
		if !f.From.Before(f.To) {
			f.From = f.To.AddDate(0, 0, -1)
		}
	}

	if !f.From.Before(f.To) {
		return f, invalid("date_from must be on or before date_to")
	}

	if f.AllTime {
		// Nothing precedes "all time"; an empty window yields no comparison.
		f.PrevFrom, f.PrevTo = f.From, f.From
	} else {
		span := f.To.Sub(f.From)
		f.PrevTo = f.From
		f.PrevFrom = f.From.Add(-span)
	}

	if id := strings.TrimSpace(q.DistrictID); id != "" {
		if _, err := uuid.Parse(id); err != nil {
			return f, invalid("district_id must be a UUID")
		}
		f.DistrictID = &id
	}

	switch g := strings.ToLower(strings.TrimSpace(q.Granularity)); g {
	case "day", "week", "month":
		f.Granularity = g
	case "", "auto":
		f.Granularity = autoGranularity(f.From, f.To)
	default:
		return f, invalid("granularity must be day, week or month")
	}

	return f, nil
}

func autoGranularity(from, to time.Time) string {
	days := to.Sub(from).Hours() / 24
	switch {
	case days <= 45:
		return "day"
	case days <= 210:
		return "week"
	default:
		return "month"
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// TruncateBucket truncates t (in report time) to the start of its bucket.
// Weeks start on Monday, matching Postgres date_trunc('week').
func TruncateBucket(t time.Time, granularity string) time.Time {
	d := startOfDay(t.In(ReportLocation))
	switch granularity {
	case "week":
		offset := (int(d.Weekday()) + 6) % 7
		return d.AddDate(0, 0, -offset)
	case "month":
		return time.Date(d.Year(), d.Month(), 1, 0, 0, 0, 0, ReportLocation)
	default:
		return d
	}
}

// Buckets lists every bucket key (YYYY-MM-DD of the bucket start) in the
// window so series can be zero-filled and charts never skip empty periods.
func Buckets(f analyticsdomain.ReportFilters) []string {
	out := []string{}
	cur := TruncateBucket(f.From, f.Granularity)
	for cur.Before(f.To) {
		out = append(out, cur.Format("2006-01-02"))
		switch f.Granularity {
		case "week":
			cur = cur.AddDate(0, 0, 7)
		case "month":
			cur = cur.AddDate(0, 1, 0)
		default:
			cur = cur.AddDate(0, 0, 1)
		}
	}
	return out
}

// Days lists every calendar day (YYYY-MM-DD) in the window.
func Days(from, to time.Time) []string {
	out := []string{}
	for cur := startOfDay(from.In(ReportLocation)); cur.Before(to); cur = cur.AddDate(0, 0, 1) {
		out = append(out, cur.Format("2006-01-02"))
	}
	return out
}

// PeriodOf echoes the resolved filters back to the client.
func PeriodOf(f analyticsdomain.ReportFilters) analyticsdomain.Period {
	return analyticsdomain.Period{
		From:        f.From,
		To:          f.To,
		PrevFrom:    f.PrevFrom,
		PrevTo:      f.PrevTo,
		Days:        int(f.To.Sub(f.From).Hours()/24 + 0.5),
		AllTime:     f.AllTime,
		Granularity: f.Granularity,
		DistrictID:  f.DistrictID,
		Timezone:    ReportTimezone,
	}
}
