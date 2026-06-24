package application

import "time"

type CreateFuelLogRequest struct {
	AmbulanceID string     `json:"ambulance_id" binding:"required,uuid"`
	FuelType    *string    `json:"fuel_type,omitempty"`
	Liters      float64    `json:"liters" binding:"required,gt=0"`
	UnitCost    *float64   `json:"unit_cost,omitempty" binding:"omitempty,gte=0"`
	Cost        *float64   `json:"cost,omitempty"`
	OdometerKM  *int       `json:"odometer_km,omitempty"`
	StationName *string    `json:"station_name,omitempty"`
	FilledAt    *time.Time `json:"filled_at,omitempty"`
	Notes       *string    `json:"notes,omitempty"`
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
}

// ConfirmFuelDispenseRequest is submitted from the public QR page by the
// person at the fuel station who actually dispensed the fuel.
type ConfirmFuelDispenseRequest struct {
	AttendantName  string     `json:"attendant_name" binding:"required"`
	AttendantPhone *string    `json:"attendant_phone,omitempty"`
	DispensedAt    *time.Time `json:"dispensed_at,omitempty"`
	Notes          *string    `json:"notes,omitempty"`
	Approved       bool       `json:"approved"`
}
