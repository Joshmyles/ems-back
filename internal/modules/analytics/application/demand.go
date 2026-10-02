package application

import (
	"math"
	"strconv"
	"time"

	analyticsdomain "dispatch/internal/modules/analytics/domain"
)

var isoWeekdays = []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
var isoWeekdayNames = []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

// forecastHorizon is how many days ahead the demand projection runs.
const forecastHorizon = 14

// z80 is the two-sided 80% normal quantile used for the projection band.
const z80 = 1.2816

// DeriveDemand zero-fills the demand series and computes the derived figures:
// rolling average, weekday totals, peaks, shares and the projection.
func DeriveDemand(d *analyticsdomain.Demand, f analyticsdomain.ReportFilters, now time.Time) {
	today := startOfDay(now.In(ReportLocation))
	end := f.To
	if tomorrow := today.AddDate(0, 0, 1); end.After(tomorrow) {
		end = tomorrow
	}

	// Daily series over every day in the window, up to today.
	byDate := map[string]analyticsdomain.DailyCount{}
	for _, c := range d.Daily {
		byDate[c.Date] = c
	}
	daily := []analyticsdomain.DailyCount{}
	for _, day := range Days(f.From, end) {
		c, ok := byDate[day]
		if !ok {
			c = analyticsdomain.DailyCount{Date: day}
		}
		daily = append(daily, c)
	}
	var total int64
	for i := range daily {
		var sum int64
		n := 0
		for j := i; j >= 0 && j > i-7; j-- {
			sum += daily[j].Count
			n++
		}
		daily[i].Rolling7 = math.Round(10*float64(sum)/float64(n)) / 10
		total += daily[i].Count
		if daily[i].Count > d.BusiestCount {
			d.BusiestCount, d.BusiestDate = daily[i].Count, daily[i].Date
		}
	}
	d.Daily = daily
	if len(daily) > 0 {
		d.AvgDaily = math.Round(100*float64(total)/float64(len(daily))) / 100
	}

	// Hour × weekday grid, zero-filled and relabelled Mon..Sun.
	grid := map[[2]int]float64{}
	for _, c := range d.HourDow {
		h, errH := strconv.Atoi(c.X)
		w, errW := strconv.Atoi(c.Y)
		if errH == nil && errW == nil && w >= 1 && w <= 7 {
			grid[[2]int{h, w}] = c.Value
		}
	}
	cells := make([]analyticsdomain.HeatCell, 0, 24*7)
	dowTotals := make([]float64, 7)
	var night, weekend, all float64
	for w := 1; w <= 7; w++ {
		for h := 0; h < 24; h++ {
			v := grid[[2]int{h, w}]
			cells = append(cells, analyticsdomain.HeatCell{X: hourLabel(h), Y: isoWeekdays[w-1], Value: v})
			dowTotals[w-1] += v
			all += v
			if h >= 19 || h < 7 {
				night += v
			}
			if w >= 6 {
				weekend += v
			}
		}
	}
	d.HourDow = cells
	d.ByDow = make([]analyticsdomain.Breakdown, 7)
	peakDow, peakDowVal := -1, 0.0
	for i, v := range dowTotals {
		d.ByDow[i] = analyticsdomain.Breakdown{Key: isoWeekdays[i], Label: isoWeekdayNames[i], Count: int64(v)}
		if v > peakDowVal {
			peakDow, peakDowVal = i, v
		}
	}
	if peakDow >= 0 {
		d.PeakDow = isoWeekdayNames[peakDow]
	}
	if all > 0 {
		d.NightShare = night / all
		d.WeekendShare = weekend / all
	}

	peakHour, peakVal := -1, int64(0)
	for _, h := range d.ByHour {
		if h.Total > peakVal {
			peakHour, peakVal = h.Hour, h.Total
		}
	}
	if peakHour >= 0 {
		d.PeakHour = &peakHour
	}

	// Project forward only when the window reaches the present.
	d.Forecast = []analyticsdomain.ForecastPoint{}
	if !f.To.Before(today.AddDate(0, 0, 1)) {
		complete := []analyticsdomain.DailyCount{}
		for _, c := range daily {
			if c.Date < today.Format("2006-01-02") {
				complete = append(complete, c)
			}
		}
		d.Forecast = Forecast(complete, today, forecastHorizon)
	}
}

// Forecast projects daily incident counts: the mean of the last 28 complete
// days scaled by weekday factors (shrunk toward 1 when a weekday has few
// observations), with a Poisson 80% band. It returns nothing with fewer than
// 14 days of history — too little to separate signal from noise.
func Forecast(history []analyticsdomain.DailyCount, start time.Time, horizon int) []analyticsdomain.ForecastPoint {
	out := []analyticsdomain.ForecastPoint{}
	if len(history) < 14 || horizon <= 0 {
		return out
	}

	recent := history
	if len(recent) > 28 {
		recent = recent[len(recent)-28:]
	}
	var level float64
	for _, c := range recent {
		level += float64(c.Count)
	}
	level /= float64(len(recent))

	seasonal := history
	if len(seasonal) > 56 {
		seasonal = seasonal[len(seasonal)-56:]
	}
	var sums [7]float64
	var obs [7]int
	var overall float64
	for _, c := range seasonal {
		t, err := time.ParseInLocation("2006-01-02", c.Date, ReportLocation)
		if err != nil {
			continue
		}
		w := (int(t.Weekday()) + 6) % 7
		sums[w] += float64(c.Count)
		obs[w]++
		overall += float64(c.Count)
	}
	overall /= float64(len(seasonal))
	var factors [7]float64
	for w := range factors {
		factors[w] = 1
		if obs[w] > 0 && overall > 0 {
			raw := (sums[w] / float64(obs[w])) / overall
			weight := float64(obs[w]) / float64(obs[w]+2)
			factors[w] = 1 + (raw-1)*weight
		}
	}

	day := startOfDay(start.In(ReportLocation))
	for i := 0; i < horizon; i++ {
		date := day.AddDate(0, 0, i)
		expected := level * factors[(int(date.Weekday())+6)%7]
		sd := math.Sqrt(expected)
		out = append(out, analyticsdomain.ForecastPoint{
			Date:     date.Format("2006-01-02"),
			Expected: round1(expected),
			Low:      round1(math.Max(0, expected-z80*sd)),
			High:     round1(expected + z80*sd),
		})
	}
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }
