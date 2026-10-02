package infrastructure

import (
	"bytes"
	"context"
	"encoding/csv"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	analyticsapp "dispatch/internal/modules/analytics/application"
	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

// These tests run every report against a real database and check that the
// numbers agree with each other. They are skipped unless
// ANALYTICS_TEST_DATABASE_URL points at a migrated EMS database, e.g.
//
//	ANALYTICS_TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5431/ems_db go test ./internal/modules/analytics/...
func testRepo(t *testing.T) *Repository {
	t.Helper()
	url := os.Getenv("ANALYTICS_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("ANALYTICS_TEST_DATABASE_URL not set")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return NewRepository(pool)
}

func windows(t *testing.T, r *Repository) map[string]analyticsdomain.ReportFilters {
	t.Helper()
	ctx := context.Background()
	start, err := r.DataStart(ctx, nil)
	if err != nil {
		t.Fatalf("data start: %v", err)
	}
	now := time.Now()
	resolve := func(q analyticsapp.ReportQuery) analyticsdomain.ReportFilters {
		f, err := analyticsapp.ResolveReportFilters(q, now, start)
		if err != nil {
			t.Fatalf("resolve %+v: %v", q, err)
		}
		return f
	}
	last30 := now.AddDate(0, 0, -29).Format("2006-01-02")
	out := map[string]analyticsdomain.ReportFilters{
		"all_time": resolve(analyticsapp.ReportQuery{}),
		"last_30":  resolve(analyticsapp.ReportQuery{DateFrom: last30}),
		"monthly":  resolve(analyticsapp.ReportQuery{Granularity: "month"}),
	}
	var district string
	if err := r.db.QueryRow(ctx, `SELECT district_id::text FROM incidents WHERE district_id IS NOT NULL
		GROUP BY 1 ORDER BY COUNT(*) DESC LIMIT 1`).Scan(&district); err == nil {
		out["district"] = resolve(analyticsapp.ReportQuery{DistrictID: district})
	}
	return out
}

func sumBreakdown(items []analyticsdomain.Breakdown) int64 {
	var n int64
	for _, b := range items {
		n += b.Count
	}
	return n
}

func TestReportsAgainstDatabase(t *testing.T) {
	r := testRepo(t)
	ctx := context.Background()

	for name, f := range windows(t, r) {
		t.Run(name, func(t *testing.T) {
			ov, err := r.Overview(ctx, f)
			if err != nil {
				t.Fatalf("overview: %v", err)
			}
			n := ov.Facts.Incidents
			for label, items := range map[string][]analyticsdomain.Breakdown{
				"priority": ov.ByPriority, "type": ov.ByType, "status": ov.ByStatus, "channel": ov.ByChannel,
			} {
				if got := sumBreakdown(items); got != n {
					t.Errorf("by_%s sums to %d, want %d incidents", label, got, n)
				}
			}
			var trendTotal int64
			for _, p := range ov.Trend {
				trendTotal += p.Total
				if p.Red+p.Orange+p.Green+p.Unprioritised != p.Total {
					t.Errorf("trend bucket %s priority split does not add up", p.Bucket)
				}
			}
			if trendTotal != n {
				t.Errorf("trend sums to %d, want %d", trendTotal, n)
			}
			if f.AllTime && (ov.Facts.PrevIncidents != 0) {
				t.Errorf("all-time report has a previous period: %d", ov.Facts.PrevIncidents)
			}

			dm, err := r.Demand(ctx, f)
			if err != nil {
				t.Fatalf("demand: %v", err)
			}
			var heat, hours int64
			for _, c := range dm.HourDow {
				heat += int64(c.Value)
			}
			for _, h := range dm.ByHour {
				hours += h.Total
			}
			if heat != n || hours != n {
				t.Errorf("demand heat=%d hours=%d, want %d", heat, hours, n)
			}

			rp, err := r.Response(ctx, f)
			if err != nil {
				t.Fatalf("response: %v", err)
			}
			if len(rp.Funnel) == 0 || rp.Funnel[0].Count != n {
				t.Errorf("funnel should start at %d reported incidents", n)
			}
			var hist int64
			for _, b := range rp.Histogram {
				hist += b.Count
			}
			if hist != rp.Dispatch.N {
				t.Errorf("histogram sums to %d, want %d dispatch samples", hist, rp.Dispatch.N)
			}
			for _, s := range rp.SLAByPriority {
				if s.Within15 > s.Within30 || s.Within30 > s.Within60 || s.Within60 > s.N {
					t.Errorf("SLA %s is not cumulative: %+v", s.Key, s)
				}
			}

			rf, err := r.Referrals(ctx, f)
			if err != nil {
				t.Fatalf("referrals: %v", err)
			}
			if rf.Totals.Incidents != n {
				t.Errorf("referrals saw %d incidents, want %d", rf.Totals.Incidents, n)
			}
			if rf.Totals.Upward+rf.Totals.Lateral+rf.Totals.Downward > rf.Totals.Transfers {
				t.Errorf("escalation split exceeds transfers")
			}

			geo, err := r.Geo(ctx, f)
			if err != nil {
				t.Fatalf("geo: %v", err)
			}
			if int64(len(geo.Points))+geo.Placeholder+geo.Missing != geo.Total {
				t.Errorf("geo points %d + placeholder %d + missing %d != %d",
					len(geo.Points), geo.Placeholder, geo.Missing, geo.Total)
			}

			cl, err := r.Clinical(ctx, f)
			if err != nil {
				t.Fatalf("clinical: %v", err)
			}
			var pyramid int64
			for _, p := range cl.Pyramid {
				pyramid += p.Male + p.Female + p.Unknown
			}
			if pyramid != n {
				t.Errorf("pyramid sums to %d, want %d", pyramid, n)
			}
			for _, v := range cl.Vitals {
				var bands int64
				for _, b := range v.Bands {
					bands += b.Count
				}
				if bands != v.Parsed || v.Parsed > v.Answered {
					t.Errorf("vital %s bands=%d parsed=%d answered=%d", v.Key, bands, v.Parsed, v.Answered)
				}
			}

			fl, err := r.Fleet(ctx, f)
			if err != nil {
				t.Fatalf("fleet: %v", err)
			}
			if fl.Totals.Active+fl.Totals.Idle != fl.Totals.FleetSize {
				t.Errorf("fleet active+idle != size")
			}
			if fl.Totals.Gini < 0 || fl.Totals.Gini > 1 {
				t.Errorf("gini out of range: %f", fl.Totals.Gini)
			}

			dq, err := r.DataQuality(ctx, f)
			if err != nil {
				t.Fatalf("data quality: %v", err)
			}
			if dq.Incidents != n {
				t.Errorf("data quality saw %d incidents, want %d", dq.Incidents, n)
			}
			for _, fq := range dq.Fields {
				if fq.Filled > fq.Total {
					t.Errorf("field %s filled %d > total %d", fq.Key, fq.Filled, fq.Total)
				}
			}

			for _, ds := range []string{analyticsapp.ExportIncidents, analyticsapp.ExportDispatches, analyticsapp.ExportFuel, analyticsapp.ExportReferrals} {
				var buf bytes.Buffer
				if err := r.Export(ctx, f, ds, &buf); err != nil {
					t.Fatalf("export %s: %v", ds, err)
				}
				records, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(buf.Bytes(), []byte("\xEF\xBB\xBF")))).ReadAll()
				if err != nil {
					t.Fatalf("export %s is not valid CSV: %v", ds, err)
				}
				if ds == analyticsapp.ExportIncidents && int64(len(records)-1) != n {
					t.Errorf("incident register has %d rows, want %d", len(records)-1, n)
				}
			}
		})
	}
}
