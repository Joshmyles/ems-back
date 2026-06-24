package domain

import "time"

type BloodRequisition struct {
	ID                    string     `json:"id"`
	IncidentID            *string    `json:"incident_id,omitempty"`
	RequestingFacilityID  *string    `json:"requesting_facility_id,omitempty"`
	PatientName           string     `json:"patient_name"`
	PatientIdentifier     string     `json:"patient_identifier"`
	ClinicalSummary       string     `json:"clinical_summary"`
	Diagnosis             string     `json:"diagnosis"`
	Indication            string     `json:"indication"`
	ParitySummary         string     `json:"parity_summary"`
	BloodGroupID          string     `json:"blood_group_id"`
	BloodGroupCode        string     `json:"blood_group_code,omitempty"`
	BloodProductID        string     `json:"blood_product_id"`
	BloodProductCode      string     `json:"blood_product_code,omitempty"`
	UnitsRequested        int        `json:"units_requested"`
	UrgencyLevel          string     `json:"urgency_level"`
	Status                string     `json:"status"`
	ReporterPhone         string     `json:"reporter_phone"`
	DestinationFacilityID *string    `json:"destination_facility_id,omitempty"`
	RequestedByUserID     *string    `json:"requested_by_user_id,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	ExpiresAt             *time.Time `json:"expires_at,omitempty"`

	// Derived display names resolved via joins (omitted when blank).
	RequestingFacilityName  string `json:"requesting_facility_name,omitempty"`
	DestinationFacilityName string `json:"destination_facility_name,omitempty"`
	RequestedByUserName     string `json:"requested_by_user_name,omitempty"`
	IncidentNumber          string `json:"incident_number,omitempty"`
}

type BloodRequisitionOffer struct {
	ID                 string     `json:"id"`
	BloodRequisitionID string     `json:"blood_requisition_id"`
	InventorySiteID    string     `json:"inventory_site_id"`
	InventorySiteName  string     `json:"inventory_site_name,omitempty"`
	BloodProductID     string     `json:"blood_product_id"`
	BloodProductCode   string     `json:"blood_product_code,omitempty"`
	BloodGroupID       string     `json:"blood_group_id"`
	BloodGroupCode     string     `json:"blood_group_code,omitempty"`
	UnitsOffered       int        `json:"units_offered"`
	ReservedUntil      *time.Time `json:"reserved_until,omitempty"`
	Notes              string     `json:"notes"`
	ContactPersonName  string     `json:"contact_person_name"`
	ContactPhone       string     `json:"contact_phone"`
	OfferedByUserID    *string    `json:"offered_by_user_id,omitempty"`
	OfferedByUserName  string     `json:"offered_by_user_name,omitempty"`
	Status             string     `json:"status"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type BloodTransportAssignment struct {
	ID                      string     `json:"id"`
	BloodRequisitionID      string     `json:"blood_requisition_id"`
	BloodRequisitionOfferID *string    `json:"blood_requisition_offer_id,omitempty"`
	VehicleType             string     `json:"vehicle_type"`
	AmbulanceID             *string    `json:"ambulance_id,omitempty"`
	DispatchAssignmentID    *string    `json:"dispatch_assignment_id,omitempty"`
	AssignedDriverUserID    *string    `json:"assigned_driver_user_id,omitempty"`
	AssignedByUserID        *string    `json:"assigned_by_user_id,omitempty"`
	PickupSiteID            *string    `json:"pickup_site_id,omitempty"`
	DestinationFacilityID   *string    `json:"destination_facility_id,omitempty"`
	Status                  string     `json:"status"`
	AssignedAt              time.Time  `json:"assigned_at"`
	CollectedAt             *time.Time `json:"collected_at,omitempty"`
	DeliveredAt             *time.Time `json:"delivered_at,omitempty"`
	Notes                   string     `json:"notes"`
}

type BloodBroadcastTarget struct {
	InventorySiteID   string  `json:"inventory_site_id"`
	InventorySiteName string  `json:"inventory_site_name"`
	DistrictName      string  `json:"district_name"`
	ContactPhone      string  `json:"contact_phone"`
	Latitude          float64 `json:"latitude"`
	Longitude         float64 `json:"longitude"`
	DistanceKM        float64 `json:"distance_km"`
	AvailableCount    int     `json:"available_count"`
}
