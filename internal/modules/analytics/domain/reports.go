package domain

import "time"

// ReportFilters is the resolved reporting window shared by every report
// endpoint. From/To are a half-open [From, To) window; PrevFrom/PrevTo is the
// immediately preceding window of equal length used for period comparison.
type ReportFilters struct {
	From        time.Time
	To          time.Time
	PrevFrom    time.Time
	PrevTo      time.Time
	AllTime     bool
	DistrictID  *string
	Granularity string // day | week | month
}

// Period describes the window a report covers, echoed back to the client.
type Period struct {
	From        time.Time `json:"from"`
	To          time.Time `json:"to"`
	PrevFrom    time.Time `json:"prev_from"`
	PrevTo      time.Time `json:"prev_to"`
	Days        int       `json:"days"`
	AllTime     bool      `json:"all_time"`
	Granularity string    `json:"granularity"`
	DistrictID  *string   `json:"district_id"`
	Timezone    string    `json:"timezone"`
}

// ReportMeta is embedded in every report payload.
type ReportMeta struct {
	GeneratedAt time.Time `json:"generated_at"`
	Period      Period    `json:"period"`
}

// HeatCell is one cell of a two-dimensional count grid (x by y).
type HeatCell struct {
	X     string  `json:"x"`
	Y     string  `json:"y"`
	Value float64 `json:"value"`
}

// Insight is a rule-generated, human-readable observation about the data.
type Insight struct {
	Key      string `json:"key"`
	Severity string `json:"severity"` // good | info | warning | critical
	Area     string `json:"area"`
	Title    string `json:"title"`
	Detail   string `json:"detail"`
}

// ── Overview ────────────────────────────────────────────────────────────────

// KPI is a headline metric with its comparison-period value and sparkline.
type KPI struct {
	Key       string    `json:"key"`
	Label     string    `json:"label"`
	Value     *float64  `json:"value"`
	Previous  *float64  `json:"previous"`
	Format    string    `json:"format"` // count | percent | minutes | currency | decimal
	UpIsGood  *bool     `json:"up_is_good"`
	Hint      string    `json:"hint"`
	Sparkline []float64 `json:"sparkline"`
}

// TrendPoint is one time bucket of incident volume and service levels.
type TrendPoint struct {
	Bucket           string   `json:"bucket"`
	Total            int64    `json:"total"`
	Red              int64    `json:"red"`
	Orange           int64    `json:"orange"`
	Green            int64    `json:"green"`
	Unprioritised    int64    `json:"unprioritised"`
	Completed        int64    `json:"completed"`
	Referrals        int64    `json:"referrals"`
	Assigned         int64    `json:"assigned"`
	AssignedWithin30 int64    `json:"assigned_within_30"`
	MedianDispatch   *float64 `json:"median_dispatch_minutes"`
	FuelCost         float64  `json:"fuel_cost"`
}

// OverviewFacts are scalar facts the service turns into KPIs and insights.
type OverviewFacts struct {
	Incidents          int64
	PrevIncidents      int64
	Red                int64
	PrevRed            int64
	Prioritised        int64
	PrevPrioritised    int64
	Completed          int64
	PrevCompleted      int64
	Closable           int64 // incidents not cancelled/rejected
	PrevClosable       int64
	Referrals          int64
	PrevReferrals      int64
	MedianDispatch     *float64
	PrevMedianDispatch *float64
	Assigned           int64
	PrevAssigned       int64
	AssignedWithin30   int64
	PrevAssignedWithin int64
	WithFeedback       int64
	PrevWithFeedback   int64
	Unclassified       int64
	FuelCost           float64
	PrevFuelCost       float64
	Dispatches         int64
	PrevDispatches     int64
	FleetSize          int64
	ActiveAmbulances   int64
	TopAmbulance       string
	TopAmbulanceShare  float64
	MaternalNeonatal   int64
	PeakHour           *int
	PeakDow            string
	GPSValid           int64
	GPSPlaceholder     int64
	OpenStale          int64 // open incidents older than 24h
}

// Overview is the executive command overview payload.
type Overview struct {
	ReportMeta
	KPIs       []KPI         `json:"kpis"`
	Trend      []TrendPoint  `json:"trend"`
	ByPriority []Breakdown   `json:"by_priority"`
	ByType     []Breakdown   `json:"by_type"`
	ByStatus   []Breakdown   `json:"by_status"`
	ByChannel  []Breakdown   `json:"by_channel"`
	ByDistrict []Breakdown   `json:"by_district"`
	Insights   []Insight     `json:"insights"`
	Facts      OverviewFacts `json:"-"`
}

// ── Demand ──────────────────────────────────────────────────────────────────

type DailyCount struct {
	Date     string  `json:"date"`
	Count    int64   `json:"count"`
	Red      int64   `json:"red"`
	Rolling7 float64 `json:"rolling_7"`
}

type HourRow struct {
	Hour   int   `json:"hour"`
	Total  int64 `json:"total"`
	Red    int64 `json:"red"`
	Orange int64 `json:"orange"`
	Green  int64 `json:"green"`
	Other  int64 `json:"other"`
}

type ForecastPoint struct {
	Date     string  `json:"date"`
	Expected float64 `json:"expected"`
	Low      float64 `json:"low"`
	High     float64 `json:"high"`
}

type Demand struct {
	ReportMeta
	HourDow      []HeatCell      `json:"hour_dow"`
	Daily        []DailyCount    `json:"daily"`
	ByHour       []HourRow       `json:"by_hour"`
	ByDow        []Breakdown     `json:"by_dow"`
	TypeByMonth  []HeatCell      `json:"type_by_bucket"`
	Forecast     []ForecastPoint `json:"forecast"`
	AvgDaily     float64         `json:"avg_daily"`
	PeakHour     *int            `json:"peak_hour"`
	PeakDow      string          `json:"peak_dow"`
	BusiestDate  string          `json:"busiest_date"`
	BusiestCount int64           `json:"busiest_count"`
	NightShare   float64         `json:"night_share"` // 19:00–06:59
	WeekendShare float64         `json:"weekend_share"`
}

// ── Response performance ────────────────────────────────────────────────────

type FunnelStage struct {
	Key           string   `json:"key"`
	Label         string   `json:"label"`
	Count         int64    `json:"count"`
	MedianMinutes *float64 `json:"median_minutes_from_previous"`
}

type Percentiles struct {
	N    int64    `json:"n"`
	Mean *float64 `json:"mean"`
	P50  *float64 `json:"p50"`
	P75  *float64 `json:"p75"`
	P90  *float64 `json:"p90"`
}

type HistogramBucket struct {
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"` // -1 = open ended
	Count int64  `json:"count"`
}

type SLARow struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	N        int64    `json:"n"`
	Within15 int64    `json:"within_15"`
	Within30 int64    `json:"within_30"`
	Within60 int64    `json:"within_60"`
	Median   *float64 `json:"median"`
	P90      *float64 `json:"p90"`
}

// BoxRow summarises a distribution per bucket. Whiskers are the 10th and 90th
// percentiles rather than min/max so a single multi-day outlier cannot flatten
// the chart.
type BoxRow struct {
	Bucket string  `json:"bucket"`
	N      int64   `json:"n"`
	P10    float64 `json:"p10"`
	Q1     float64 `json:"q1"`
	Median float64 `json:"median"`
	Q3     float64 `json:"q3"`
	P90    float64 `json:"p90"`
}

type PersonRow struct {
	UserID        string     `json:"user_id"`
	Name          string     `json:"name"`
	Role          string     `json:"role"`
	Count         int64      `json:"count"`
	MedianMinutes *float64   `json:"median_minutes"`
	Within30      int64      `json:"within_30"`
	LastAt        *time.Time `json:"last_at"`
}

type SlowCase struct {
	IncidentID     string    `json:"incident_id"`
	IncidentNumber string    `json:"incident_number"`
	ReportedAt     time.Time `json:"reported_at"`
	Minutes        float64   `json:"minutes"`
	Priority       string    `json:"priority"`
	Type           string    `json:"type"`
	Status         string    `json:"status"`
}

type ResponsePerformance struct {
	ReportMeta
	Funnel        []FunnelStage     `json:"funnel"`
	Dispatch      Percentiles       `json:"dispatch"`
	Closure       Percentiles       `json:"closure"`
	SceneArrival  Percentiles       `json:"scene_arrival"`
	Histogram     []HistogramBucket `json:"histogram"`
	SLAByPriority []SLARow          `json:"sla_by_priority"`
	SLAByChannel  []SLARow          `json:"sla_by_channel"`
	Box           []BoxRow          `json:"box"`
	Dispatchers   []PersonRow       `json:"dispatchers"`
	Slowest       []SlowCase        `json:"slowest"`
}

// ── Referral network ────────────────────────────────────────────────────────

type Flow struct {
	SourceID    string `json:"source_id"`
	Source      string `json:"source"`
	SourceLevel string `json:"source_level"`
	TargetID    string `json:"target_id"`
	Target      string `json:"target"`
	TargetLevel string `json:"target_level"`
	Count       int64  `json:"count"`
	Critical    int64  `json:"critical"`
}

type FacilityRow struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Level     string   `json:"level"`
	District  string   `json:"district"`
	Sent      int64    `json:"sent"`
	Received  int64    `json:"received"`
	Critical  int64    `json:"critical"`
	Latitude  *float64 `json:"latitude"`
	Longitude *float64 `json:"longitude"`
}

type ReferralTotals struct {
	Incidents       int64 `json:"incidents"`
	Transfers       int64 `json:"transfers"` // origin and destination both recorded
	WithOrigin      int64 `json:"with_origin"`
	WithDestination int64 `json:"with_destination"`
	Origins         int64 `json:"origins"`
	Destinations    int64 `json:"destinations"`
	Upward          int64 `json:"upward"`
	Lateral         int64 `json:"lateral"`
	Downward        int64 `json:"downward"`
	Critical        int64 `json:"critical"`
	ToNational      int64 `json:"to_national"`
}

type ReferralNetwork struct {
	ReportMeta
	Totals      ReferralTotals `json:"totals"`
	Flows       []Flow         `json:"flows"`
	LevelFlows  []Flow         `json:"level_flows"`
	LevelMatrix []HeatCell     `json:"level_matrix"`
	Facilities  []FacilityRow  `json:"facilities"`
	Reasons     []Breakdown    `json:"reasons"`
}

// ── Geography ───────────────────────────────────────────────────────────────

type GeoPoint struct {
	ID         string    `json:"id"`
	Number     string    `json:"number"`
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Priority   string    `json:"priority"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	ReportedAt time.Time `json:"reported_at"`
}

type Geo struct {
	ReportMeta
	Points      []GeoPoint    `json:"points"`
	Facilities  []FacilityRow `json:"facilities"`
	Flows       []Flow        `json:"flows"`
	Placeholder int64         `json:"placeholder"` // 0,0 or out-of-country coordinates
	Missing     int64         `json:"missing"`
	Total       int64         `json:"total"`
}

// ── Clinical ────────────────────────────────────────────────────────────────

type RedFlag struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Positive int64  `json:"positive"`
	Answered int64  `json:"answered"`
}

type VitalBand struct {
	Label string `json:"label"`
	Tone  string `json:"tone"` // good | warning | serious | critical | neutral
	Count int64  `json:"count"`
}

type VitalSummary struct {
	Key      string      `json:"key"`
	Label    string      `json:"label"`
	Unit     string      `json:"unit"`
	Answered int64       `json:"answered"`
	Parsed   int64       `json:"parsed"`
	Median   *float64    `json:"median"`
	Bands    []VitalBand `json:"bands"`
}

type PyramidRow struct {
	AgeGroup string `json:"age_group"`
	Male     int64  `json:"male"`
	Female   int64  `json:"female"`
	Unknown  int64  `json:"unknown"`
}

type TypeRow struct {
	Key            string   `json:"key"`
	Label          string   `json:"label"`
	Count          int64    `json:"count"`
	Critical       int64    `json:"critical"`
	Completed      int64    `json:"completed"`
	MedianDispatch *float64 `json:"median_dispatch_minutes"`
}

type MNHSummary struct {
	Maternal             int64    `json:"maternal"`
	Neonatal             int64    `json:"neonatal"`
	Share                float64  `json:"share"`
	Critical             int64    `json:"critical"`
	MedianDispatch       *float64 `json:"median_dispatch_minutes"`
	HypertensiveMaternal int64    `json:"hypertensive_maternal"` // SBP >= 140 or DBP >= 90
	SevereHypertension   int64    `json:"severe_hypertension"`   // SBP >= 160 or DBP >= 110
	MaternalWithBP       int64    `json:"maternal_with_bp"`
	NeonatalHypothermia  int64    `json:"neonatal_hypothermia"` // temp < 36.5
	NeonatalWithTemp     int64    `json:"neonatal_with_temp"`
}

type Clinical struct {
	ReportMeta
	Incidents      int64             `json:"incidents"`
	Triaged        int64             `json:"triaged"`
	AutoDispatch   int64             `json:"auto_dispatch_eligible"`
	OnOxygen       int64             `json:"on_oxygen"`
	RedFlags       []RedFlag         `json:"red_flags"`
	Vitals         []VitalSummary    `json:"vitals"`
	Pyramid        []PyramidRow      `json:"pyramid"`
	TypePriority   []HeatCell        `json:"type_priority"`
	ByType         []TypeRow         `json:"by_type"`
	ScoreHistogram []HistogramBucket `json:"score_histogram"`
	MNH            MNHSummary        `json:"mnh"`
	Outcomes       []Breakdown       `json:"outcomes"`
}

// ── Fleet ───────────────────────────────────────────────────────────────────

type AmbulanceRow struct {
	ID              string     `json:"id"`
	Plate           string     `json:"plate"`
	Code            string     `json:"code"`
	Category        string     `json:"category"`
	Status          string     `json:"status"`
	Readiness       string     `json:"readiness"`
	Dispatches      int64      `json:"dispatches"`
	Share           float64    `json:"share"`
	ActiveDays      int64      `json:"active_days"`
	LastDispatchAt  *time.Time `json:"last_dispatch_at"`
	FuelLogs        int64      `json:"fuel_logs"`
	Liters          float64    `json:"liters"`
	FuelCost        float64    `json:"fuel_cost"`
	ValidKm         float64    `json:"valid_km"`
	KmPerLiter      *float64   `json:"km_per_liter"`
	OdometerOK      int64      `json:"odometer_ok"`
	OdometerChecks  int64      `json:"odometer_checks"`
	CostPerDispatch *float64   `json:"cost_per_dispatch"`
	Anomalies       int64      `json:"anomalies"`
}

type FuelAnomaly struct {
	LogID       string    `json:"log_id"`
	Ambulance   string    `json:"ambulance"`
	FilledAt    time.Time `json:"filled_at"`
	Liters      float64   `json:"liters"`
	Cost        float64   `json:"cost"`
	OdometerKm  *int64    `json:"odometer_km"`
	KmSinceLast *int64    `json:"km_since_last"`
	Reason      string    `json:"reason"`
	Code        string    `json:"code"`
	Severity    string    `json:"severity"`
}

type FleetTotals struct {
	FleetSize           int64    `json:"fleet_size"`
	Active              int64    `json:"active"`
	Idle                int64    `json:"idle"`
	Dispatches          int64    `json:"dispatches"`
	Liters              float64  `json:"liters"`
	FuelCost            float64  `json:"fuel_cost"`
	CostPerDispatch     *float64 `json:"cost_per_dispatch"`
	KmPerLiter          *float64 `json:"km_per_liter"`
	TopShare            float64  `json:"top_share"`
	Gini                float64  `json:"gini"`
	Anomalies           int64    `json:"anomalies"`
	OdometerReliability *float64 `json:"odometer_reliability"`
}

type FleetPerformance struct {
	ReportMeta
	Totals     FleetTotals    `json:"totals"`
	Ambulances []AmbulanceRow `json:"ambulances"`
	Load       []HeatCell     `json:"load"` // ambulance x bucket dispatches
	Crew       []PersonRow    `json:"crew"`
	Anomalies  []FuelAnomaly  `json:"anomalies"`
	AnomalyMix []Breakdown    `json:"anomaly_mix"`
	PriceTrend []PricePoint   `json:"price_trend"`
}

type PricePoint struct {
	Bucket    string   `json:"bucket"`
	FuelType  string   `json:"fuel_type"`
	UnitPrice *float64 `json:"unit_price"`
	Liters    float64  `json:"liters"`
}

// ── Data quality ────────────────────────────────────────────────────────────

type FieldQuality struct {
	Key            string  `json:"key"`
	Label          string  `json:"label"`
	Group          string  `json:"group"`
	Filled         int64   `json:"filled"`
	Total          int64   `json:"total"`
	Target         float64 `json:"target"`
	Weight         float64 `json:"weight"`
	Recommendation string  `json:"recommendation"`
}

type QualityIssue struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Count    int64  `json:"count"`
	Severity string `json:"severity"`
	Detail   string `json:"detail"`
}

type QualityTrendPoint struct {
	Bucket     string   `json:"bucket"`
	Total      int64    `json:"total"`
	Priority   *float64 `json:"priority"`
	Classified *float64 `json:"classified"`
	Triage     *float64 `json:"triage"`
	GPS        *float64 `json:"gps"`
	Facility   *float64 `json:"facility"`
	Feedback   *float64 `json:"feedback"`
}

type ChannelQuality struct {
	Channel string  `json:"channel"`
	Total   int64   `json:"total"`
	Score   float64 `json:"score"`
}

type ActivityPoint struct {
	Bucket   string `json:"bucket"`
	Users    int64  `json:"users"`
	Sessions int64  `json:"sessions"`
}

type NotificationRow struct {
	Channel string `json:"channel"`
	Total   int64  `json:"total"`
	Read    int64  `json:"read"`
	Sent    int64  `json:"sent"`
	Pending int64  `json:"pending"`
	Failed  int64  `json:"failed"`
}

type Adoption struct {
	ActiveUsers   int64             `json:"active_users"`
	TotalUsers    int64             `json:"total_users"`
	Activity      []ActivityPoint   `json:"activity"`
	Notifications []NotificationRow `json:"notifications"`
	UsersByRole   []Breakdown       `json:"users_by_role"`
}

type DataQuality struct {
	ReportMeta
	Score     float64             `json:"score"`
	Grade     string              `json:"grade"`
	Incidents int64               `json:"incidents"`
	Fields    []FieldQuality      `json:"fields"`
	Issues    []QualityIssue      `json:"issues"`
	Trend     []QualityTrendPoint `json:"trend"`
	ByChannel []ChannelQuality    `json:"by_channel"`
	Adoption  Adoption            `json:"adoption"`
}
