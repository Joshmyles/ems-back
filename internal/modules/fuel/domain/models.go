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

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FundingSource is a pot of money (donor/government allocation) that fuel
// purchases can draw from. Spent is the sum of linked fuel log costs and
// Remaining = Amount - Spent, both computed at read time.
type FundingSource struct {
	ID               string    `json:"id"`
	OrganisationName string    `json:"organisation_name"`
	FundingDate      time.Time `json:"funding_date"`
	Amount           float64   `json:"amount"`
	Notes            *string   `json:"notes,omitempty"`
	Spent            float64   `json:"spent"`
	Remaining        float64   `json:"remaining"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
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
