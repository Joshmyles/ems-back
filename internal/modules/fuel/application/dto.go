package application

import "time"

type CreateFuelLogRequest struct {
	AmbulanceID     string     `json:"ambulance_id" binding:"required,uuid"`
	FuelType        *string    `json:"fuel_type,omitempty"`
	Liters          float64    `json:"liters" binding:"required,gt=0"`
	UnitCost        *float64   `json:"unit_cost,omitempty" binding:"omitempty,gte=0"`
	Cost            *float64   `json:"cost,omitempty"`
	OdometerKM      *int       `json:"odometer_km,omitempty"`
	StationName     *string    `json:"station_name,omitempty"`
	FilledAt        *time.Time `json:"filled_at,omitempty"`
	Notes           *string    `json:"notes,omitempty"`
	FundingSourceID *string    `json:"funding_source_id,omitempty" binding:"omitempty,uuid"`
}

type UpdateFuelLogRequest struct {
	FuelType    *string    `json:"fuel_type,omitempty"`
	Liters      *float64   `json:"liters,omitempty"`
	UnitCost    *float64   `json:"unit_cost,omitempty" binding:"omitempty,gte=0"`
	Cost        *float64   `json:"cost,omitempty"`
	OdometerKM  *int       `json:"odometer_km,omitempty"`
	StationName *string    `json:"station_name,omitempty"`
	FilledAt    *time.Time `json:"filled_at,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
	// Empty string clears the link; a UUID re-points it.
	FundingSourceID *string `json:"funding_source_id,omitempty"`
}

// CreateFundingSourceRequest registers a pot of fuel money.
type CreateFundingSourceRequest struct {
	OrganisationName string  `json:"organisation_name" binding:"required"`
	FundingDate      *string `json:"funding_date,omitempty" binding:"omitempty,datetime=2006-01-02"`
	Amount           float64 `json:"amount" binding:"required,gte=0"`
	Notes            *string `json:"notes,omitempty"`
}

// UpdateFundingSourceRequest edits a funding source's details. All fields are
// optional; only the supplied ones change. Amount edits the base allocation
// (use a top-up to add money without rewriting history).
type UpdateFundingSourceRequest struct {
	OrganisationName *string  `json:"organisation_name,omitempty"`
	FundingDate      *string  `json:"funding_date,omitempty" binding:"omitempty,datetime=2006-01-02"`
	Amount           *float64 `json:"amount,omitempty" binding:"omitempty,gte=0"`
	Notes            *string  `json:"notes,omitempty"`
}

// CreateFundingTopupRequest adds money to an existing funding source.
type CreateFundingTopupRequest struct {
	Amount    float64 `json:"amount" binding:"required,gt=0"`
	TopupDate *string `json:"topup_date,omitempty" binding:"omitempty,datetime=2006-01-02"`
	Notes     *string `json:"notes,omitempty"`
}

// ConfirmFuelDispenseRequest is submitted from the public QR page by the
// person at the fuel station who actually dispensed the fuel.
type ConfirmFuelDispenseRequest struct {
	AttendantName string `json:"attendant_name" binding:"required"`
	// Mandatory at confirmation: the attendant records the actual odometer
	// reading at the pump even though it is optional when the log is created.
	OdometerKM     *int       `json:"odometer_km" binding:"required,min=0"`
	AttendantPhone *string    `json:"attendant_phone,omitempty"`
	DispensedAt    *time.Time `json:"dispensed_at,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	Approved       bool       `json:"approved"`
}
