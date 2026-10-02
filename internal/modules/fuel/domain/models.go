package domain

import "time"

type FuelLog struct {
	ID          string    `json:"id"`
	AmbulanceID string    `json:"ambulance_id"`
	FuelType    *string   `json:"fuel_type,omitempty"`
	Liters      float64   `json:"liters"`
	UnitCost    *float64  `json:"unit_cost,omitempty"`
	Cost        *float64  `json:"cost,omitempty"`
	OdometerKM  *int      `json:"odometer_km,omitempty"`
	StationName *string   `json:"station_name,omitempty"`
	FilledAt    time.Time `json:"filled_at"`
	FilledBy    *string   `json:"filled_by,omitempty"`
	Notes       *string   `json:"notes,omitempty"`

	// Optional funding source this purchase draws from.
	FundingSourceID   *string `json:"funding_source_id,omitempty"`
	FundingSourceName *string `json:"funding_source_name,omitempty"`

	// QR-based public verification.
	PublicToken       string     `json:"public_token"`
	DispensedAt       *time.Time `json:"dispensed_at,omitempty"`
	DispenseConfirmed bool       `json:"dispense_confirmed"`
	AttendantName     *string    `json:"attendant_name,omitempty"`
	AttendantPhone    *string    `json:"attendant_phone,omitempty"`
	AttendantNotes    *string    `json:"attendant_notes,omitempty"`
	ConfirmedAt       *time.Time `json:"confirmed_at,omitempty"`

	// Display fields resolved from related tables when reading.
	AmbulancePlate *string `json:"ambulance_plate,omitempty"`
	AmbulanceCode  *string `json:"ambulance_code,omitempty"`
	FilledByName   *string `json:"filled_by_name,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FuelLogSummary aggregates fuel logs across all records (or a driver's own
// ambulances) so the board is not limited to the current page.
type FuelLogSummary struct {
	Total     int64 `json:"total"`
	Pending   int64 `json:"pending"`
	Confirmed int64 `json:"confirmed"`
	// PendingOverdue counts pending logs filled more than 24 hours ago.
	PendingOverdue  int64      `json:"pending_overdue"`
	OldestPendingAt *time.Time `json:"oldest_pending_at,omitempty"`

	// Recent covers the last 30 days; Previous the 30 days before that.
	Recent   FuelPeriodTotals `json:"recent"`
	Previous FuelPeriodTotals `json:"previous"`

	FuelTypes []FuelTypeCount `json:"fuel_types"`
	// Weekly is the last 12 weeks (oldest first), including empty weeks.
	Weekly []FuelWeek `json:"weekly"`
	// FundingBurn is spend per funding source over the last 30 days.
	FundingBurn []FundingBurn `json:"funding_burn"`
}

type FuelPeriodTotals struct {
	Logs   int64   `json:"logs"`
	Liters float64 `json:"liters"`
	Cost   float64 `json:"cost"`
	// PricedLiters only counts logs with a cost, so Cost / PricedLiters is
	// the average price per litre.
	PricedLiters float64 `json:"priced_liters"`
}

type FuelTypeCount struct {
	FuelType string `json:"fuel_type"`
	Count    int64  `json:"count"`
}

type FuelWeek struct {
	WeekStart time.Time `json:"week_start"`
	Logs      int64     `json:"logs"`
	Liters    float64   `json:"liters"`
	Cost      float64   `json:"cost"`
}

type FundingBurn struct {
	FundingSourceID string  `json:"funding_source_id"`
	Cost30d         float64 `json:"cost_30d"`
}

// FundingSource is a pot of money (donor/government allocation) that fuel
// purchases can draw from. It is a living budget: the initial Amount can be
// extended with top-ups. All derived figures are computed at read time:
//
//	TotalFunded = Amount + TopupTotal
//	Remaining   = TotalFunded - Spent (sum of linked fuel-log costs)
type FundingSource struct {
	ID               string    `json:"id"`
	OrganisationName string    `json:"organisation_name"`
	FundingDate      time.Time `json:"funding_date"`
	Amount           float64   `json:"amount"`
	Notes            *string   `json:"notes,omitempty"`
	Spent            float64   `json:"spent"`
	TopupTotal       float64   `json:"topup_total"`
	TopupCount       int       `json:"topup_count"`
	TotalFunded      float64   `json:"total_funded"`
	Remaining        float64   `json:"remaining"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// FundingTopup is a single addition of money to a funding source.
type FundingTopup struct {
	ID              string    `json:"id"`
	FundingSourceID string    `json:"funding_source_id"`
	Amount          float64   `json:"amount"`
	TopupDate       time.Time `json:"topup_date"`
	Notes           *string   `json:"notes,omitempty"`
	CreatedBy       *string   `json:"created_by,omitempty"`
	CreatedByName   *string   `json:"created_by_name,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// CrewMember is a person attached to the ambulance via its active crew assignment.
type CrewMember struct {
	Role  string  `json:"role"`
	Name  string  `json:"name"`
	Phone *string `json:"phone,omitempty"`
}

// FuelLogPublicView is the payload exposed when a fuel log QR code is scanned.
type FuelLogPublicView struct {
	FuelLog        FuelLog      `json:"fuel_log"`
	AmbulancePlate string       `json:"ambulance_plate"`
	AmbulanceCode  *string      `json:"ambulance_code,omitempty"`
	AmbulanceMake  *string      `json:"ambulance_make,omitempty"`
	AmbulanceModel *string      `json:"ambulance_model,omitempty"`
	LoggedByName   *string      `json:"logged_by_name,omitempty"`
	Crew           []CrewMember `json:"crew"`
}
