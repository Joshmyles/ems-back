package application

import (
	"errors"
	"strings"
	"testing"
	"time"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

var testNow = time.Date(2026, 10, 2, 9, 30, 0, 0, ReportLocation)

func TestResolveReportFiltersWindow(t *testing.T) {
	f, err := ResolveReportFilters(ReportQuery{DateFrom: "2026-09-01", DateTo: "2026-09-30"}, testNow, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := f.From.Format(time.RFC3339); got != "2026-09-01T00:00:00+03:00" {
		t.Errorf("from = %s", got)
	}
	// date_to is inclusive: the window ends at the start of the next day.
	if got := f.To.Format(time.RFC3339); got != "2026-10-01T00:00:00+03:00" {
		t.Errorf("to = %s", got)
	}
	// The comparison window is the 30 days immediately before.
	if got := f.PrevFrom.Format("2006-01-02"); got != "2026-08-02" {
		t.Errorf("prev from = %s", got)
	}
	if !f.PrevTo.Equal(f.From) {
		t.Errorf("prev window must end where the window starts")
	}
	if f.Granularity != "day" || f.AllTime {
		t.Errorf("granularity=%s allTime=%v", f.Granularity, f.AllTime)
	}
}

func TestResolveReportFiltersAllTime(t *testing.T) {
	start := time.Date(2026, 6, 12, 13, 7, 0, 0, time.UTC)
	f, err := ResolveReportFilters(ReportQuery{}, testNow, &start)
	if err != nil {
		t.Fatal(err)
	}
	if !f.AllTime {
		t.Fatal("expected an all-time window")
	}
	if got := f.From.Format("2006-01-02"); got != "2026-06-12" {
		t.Errorf("all-time window should start on the first incident's day, got %s", got)
	}
	if got := f.To.Format("2006-01-02"); got != "2026-10-03" {
		t.Errorf("all-time window should run through today, got %s", got)
	}
	if !f.PrevFrom.Equal(f.PrevTo) {
		t.Errorf("all-time reports must have an empty comparison window")
	}
	if f.Granularity != "week" {
		t.Errorf("~16 weeks should bucket weekly, got %s", f.Granularity)
	}
}

func TestResolveReportFiltersRejectsBadInput(t *testing.T) {
	cases := []ReportQuery{
		{DateFrom: "01/09/2026"},
		{DateTo: "tomorrow"},
		{DateFrom: "2026-09-10", DateTo: "2026-09-01"},
		{DistrictID: "kampala"},
		{Granularity: "hour"},
	}
	for _, q := range cases {
		if _, err := ResolveReportFilters(q, testNow, nil); !errors.Is(err, ErrInvalidReportQuery) {
			t.Errorf("%+v: expected ErrInvalidReportQuery, got %v", q, err)
		}
	}
}

func TestBucketsAreContiguousAndMondayAligned(t *testing.T) {
	f, _ := ResolveReportFilters(ReportQuery{DateFrom: "2026-09-03", DateTo: "2026-09-30", Granularity: "week"}, testNow, nil)
	got := Buckets(f)
	want := []string{"2026-08-31", "2026-09-07", "2026-09-14", "2026-09-21", "2026-09-28"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("weekly buckets = %v, want %v", got, want)
	}
	f.Granularity = "month"
	if got := Buckets(f); len(got) != 1 || got[0] != "2026-09-01" {
		t.Errorf("monthly buckets = %v", got)
	}
}

func TestForecastFollowsWeekdayPattern(t *testing.T) {
	// Six weeks where Thursdays are three times busier than other days.
	history := []analyticsdomain.DailyCount{}
	day := time.Date(2026, 8, 17, 0, 0, 0, 0, ReportLocation) // a Monday
	for i := 0; i < 42; i++ {
		d := day.AddDate(0, 0, i)
		n := int64(2)
		if d.Weekday() == time.Thursday {
			n = 6
		}
		history = append(history, analyticsdomain.DailyCount{Date: d.Format("2006-01-02"), Count: n})
	}
	start := day.AddDate(0, 0, 42) // the following Monday
	fc := Forecast(history, start, 7)
	if len(fc) != 7 {
		t.Fatalf("want 7 forecast days, got %d", len(fc))
	}
	monday, thursday := fc[0], fc[3]
	if thursday.Expected <= monday.Expected*2 {
		t.Errorf("Thursday (%.1f) should be projected well above Monday (%.1f)", thursday.Expected, monday.Expected)
	}
	for _, p := range fc {
		if p.Low > p.Expected || p.High < p.Expected || p.Low < 0 {
			t.Errorf("band does not bracket the estimate: %+v", p)
		}
	}
	if got := Forecast(history[:10], start, 7); len(got) != 0 {
		t.Errorf("fewer than 14 days of history must not be projected")
	}
}

func TestDeriveDemandZeroFillsAndFindsPeaks(t *testing.T) {
	f, _ := ResolveReportFilters(ReportQuery{DateFrom: "2026-09-28", DateTo: "2026-10-01"}, testNow, nil)
	d := &analyticsdomain.Demand{
		Daily:   []analyticsdomain.DailyCount{{Date: "2026-09-29", Count: 5}},
		HourDow: []analyticsdomain.HeatCell{{X: "13", Y: "2", Value: 4}, {X: "02", Y: "6", Value: 1}},
		ByHour:  []analyticsdomain.HourRow{{Hour: 13, Total: 4}, {Hour: 2, Total: 1}},
	}
	DeriveDemand(d, f, testNow)
	if len(d.Daily) != 4 {
		t.Fatalf("expected 4 zero-filled days, got %d", len(d.Daily))
	}
	if len(d.HourDow) != 24*7 {
		t.Errorf("expected a full 24×7 grid, got %d cells", len(d.HourDow))
	}
	if d.PeakHour == nil || *d.PeakHour != 13 || d.PeakDow != "Tuesday" {
		t.Errorf("peak = %v / %s", d.PeakHour, d.PeakDow)
	}
	if d.BusiestDate != "2026-09-29" || d.WeekendShare != 0.2 || d.NightShare != 0.2 {
		t.Errorf("busiest=%s weekend=%.2f night=%.2f", d.BusiestDate, d.WeekendShare, d.NightShare)
	}
	if len(d.Forecast) != 0 {
		t.Errorf("a historical window must not be projected")
	}
}

func TestBuildInsightsFlagsTheAuditFindings(t *testing.T) {
	median, prevMedian := 39.0, 25.0
	peak := 13
	facts := analyticsdomain.OverviewFacts{
		Incidents: 282, PrevIncidents: 150,
		Red: 183, Prioritised: 209,
		Completed: 224, WithFeedback: 3,
		Assigned: 263, AssignedWithin30: 120, PrevAssigned: 100, PrevAssignedWithin: 70,
		MedianDispatch: &median, PrevMedianDispatch: &prevMedian,
		Unclassified: 95, MaternalNeonatal: 97,
		FleetSize: 9, ActiveAmbulances: 5, TopAmbulance: "UG 3201582", TopAmbulanceShare: 0.38,
		GPSValid: 54, GPSPlaceholder: 223,
		PeakHour: &peak, PeakDow: "Thursday",
	}
	insights := BuildInsights(facts, false)
	keys := map[string]analyticsdomain.Insight{}
	for _, in := range insights {
		keys[in.Key] = in
	}
	for _, want := range []string{"feedback_gap", "dispatch_slower", "dispatch_sla", "fleet_concentration",
		"idle_fleet", "missing_priority", "unclassified", "gps_placeholder", "mnh_burden", "demand_peak", "volume_change"} {
		if _, ok := keys[want]; !ok {
			t.Errorf("missing insight %q", want)
		}
	}
	if insights[0].Severity != "critical" {
		t.Errorf("most severe insight should come first, got %s", insights[0].Severity)
	}
	if allTime := BuildInsights(facts, true); containsKey(allTime, "volume_change") {
		t.Errorf("all-time reports must not compare against a previous period")
	}
}

func containsKey(in []analyticsdomain.Insight, key string) bool {
	for _, i := range in {
		if i.Key == key {
			return true
		}
	}
	return false
}

func TestBuildKPIsOmitsComparisonForAllTime(t *testing.T) {
	kpis := BuildKPIs(analyticsdomain.OverviewFacts{Incidents: 10, PrevIncidents: 4}, []analyticsdomain.TrendPoint{{Total: 3}, {Total: 7}}, true)
	if kpis[0].Previous != nil {
		t.Errorf("all-time KPI should have no previous value")
	}
	if got := kpis[0].Sparkline; len(got) != 2 || got[1] != 7 {
		t.Errorf("sparkline = %v", got)
	}
	withPrev := BuildKPIs(analyticsdomain.OverviewFacts{Incidents: 10, PrevIncidents: 4}, nil, false)
	if withPrev[0].Previous == nil || *withPrev[0].Previous != 4 {
		t.Errorf("expected previous incidents = 4")
	}
}

func TestScoreQuality(t *testing.T) {
	fields := []analyticsdomain.FieldQuality{
		{Filled: 100, Total: 100, Target: 1, Weight: 3},
		{Filled: 40, Total: 100, Target: 0.8, Weight: 1}, // 50% of target
		{Filled: 0, Total: 0, Target: 1, Weight: 5},      // no denominator: ignored
	}
	score, grade := ScoreQuality(fields)
	if score != 87.5 || grade != "B" {
		t.Errorf("score=%.1f grade=%s, want 87.5 B", score, grade)
	}
	if _, g := ScoreQuality(nil); g != "–" {
		t.Errorf("empty input should have no grade")
	}
}
