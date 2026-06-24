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
