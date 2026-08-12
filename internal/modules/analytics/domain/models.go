package domain

import "time"

// Filters narrows the analytics aggregation by reporting window and district.
type Filters struct {
	DateFrom   *time.Time
	DateTo     *time.Time
	DistrictID *string
}

// Breakdown is a single labelled bucket in a distribution (e.g. one status,
// one priority level, one patient sex).
type Breakdown struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Count int64  `json:"count"`
}

// Totals holds the headline system-wide counters.
type Totals struct {
	Incidents           int64 `json:"incidents"`
	Open                int64 `json:"open"`
	Assigned            int64 `json:"assigned"`
	InTransport         int64 `json:"in_transport"`
	Completed           int64 `json:"completed"`
	Cancelled           int64 `json:"cancelled"`
	Referrals           int64 `json:"referrals"`
	Transfers           int64 `json:"transfers"`
	Critical            int64 `json:"critical"`
	Verified            int64 `json:"verified"`
	PendingVerification int64 `json:"pending_verification"`
}

// ResponseTimes summarises operational turnaround in minutes. Pointers are nil
// when no incident in the window has reached the relevant milestone.
type ResponseTimes struct {
	AvgMinutesToAssignment *float64 `json:"avg_minutes_to_assignment"`
	AvgMinutesToClosure    *float64 `json:"avg_minutes_to_closure"`
}

// PatientReport consolidates the patient-centric view of the window.
type PatientReport struct {
	TotalPatients    int64       `json:"total_patients"`
	Transported      int64       `json:"transported"`
	OutcomesReported int64       `json:"outcomes_reported"`
	BySex            []Breakdown `json:"by_sex"`
	ByAgeGroup       []Breakdown `json:"by_age_group"`
	Outcomes         []Breakdown `json:"outcomes"`
}

// DistrictRow is the per-district rollup used for the district-level dashboard.
type DistrictRow struct {
	DistrictID string `json:"district_id"`
	District   string `json:"district"`
	Total      int64  `json:"total"`
	Referrals  int64  `json:"referrals"`
	Transfers  int64  `json:"transfers"`
	Critical   int64  `json:"critical"`
	Completed  int64  `json:"completed"`
}

// Summary is the full consolidated analytics payload returned to the client.
type Summary struct {
	GeneratedAt   time.Time      `json:"generated_at"`
	Filters       map[string]any `json:"filters"`
	Totals        Totals         `json:"totals"`
	ResponseTimes ResponseTimes  `json:"response_times"`
	PatientReport PatientReport  `json:"patient_report"`
	ByStatus      []Breakdown    `json:"by_status"`
	ByPriority    []Breakdown    `json:"by_priority"`
	BySeverity    []Breakdown    `json:"by_severity"`
	ByType        []Breakdown    `json:"by_type"`
	ByDistrict    []DistrictRow  `json:"by_district"`
}

// FuelBucket is one labelled slice of fuel consumption (e.g. one ambulance).
type FuelBucket struct {
	Key    string  `json:"key"`
	Label  string  `json:"label"`
	Liters float64 `json:"liters"`
	Cost   float64 `json:"cost"`
	Logs   int64   `json:"logs"`
}

// FuelMonthly is fuel consumption aggregated per calendar month (YYYY-MM).
type FuelMonthly struct {
	Month  string  `json:"month"`
	Liters float64 `json:"liters"`
	Cost   float64 `json:"cost"`
	Logs   int64   `json:"logs"`
}

// FuelFundingSourceReport shows a funder how their allocation is being spent.
type FuelFundingSourceReport struct {
	ID               string        `json:"id"`
	OrganisationName string        `json:"organisation_name"`
	FundingDate      time.Time     `json:"funding_date"`
	Amount           float64       `json:"amount"`
	Spent            float64       `json:"spent"`
	Remaining        float64       `json:"remaining"`
	ByAmbulance      []FuelBucket  `json:"by_ambulance"`
	Monthly          []FuelMonthly `json:"monthly"`
}

// FuelTotals holds fleet-wide fuel counters for the reporting window.
type FuelTotals struct {
	Logs         int64   `json:"logs"`
	Liters       float64 `json:"liters"`
	Cost         float64 `json:"cost"`
	FundedCost   float64 `json:"funded_cost"`
	UnfundedCost float64 `json:"unfunded_cost"`
}

// FuelAnalytics is the consolidated fuel consumption report.
type FuelAnalytics struct {
	GeneratedAt    time.Time                 `json:"generated_at"`
	Totals         FuelTotals                `json:"totals"`
	ByAmbulance    []FuelBucket              `json:"by_ambulance"`
	Monthly        []FuelMonthly             `json:"monthly"`
	FundingSources []FuelFundingSourceReport `json:"funding_sources"`
}
