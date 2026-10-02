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

	// Workflow pointers: the accepted offer and the latest live pickup.
	AcceptedOfferID    *string `json:"accepted_offer_id,omitempty"`
	PickupAssignmentID *string `json:"pickup_assignment_id,omitempty"`
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

// BloodRequisitionSummary aggregates requisitions across the whole table so
// dashboards are not limited to the current page.
type BloodRequisitionSummary struct {
	Total     int64            `json:"total"`
	ByStatus  map[string]int64 `json:"by_status"`
	ByUrgency map[string]int64 `json:"by_urgency"`
	// ByStatusUrgency breaks each status down by urgency level.
	ByStatusUrgency map[string]map[string]int64 `json:"by_status_urgency"`
	// Live counts cover requisitions that still need work (not delivered,
	// declined, cancelled or expired).
	LiveTotal     int64 `json:"live_total"`
	LiveEmergency int64 `json:"live_emergency"`
	UnitsPending  int64 `json:"units_pending"`
}

// BloodRequisitionEvent is one entry of a requisition's status history.
type BloodRequisitionEvent struct {
	ID             string    `json:"id"`
	PreviousStatus string    `json:"previous_status,omitempty"`
	NewStatus      string    `json:"new_status"`
	Notes          string    `json:"notes,omitempty"`
	ActorUserID    *string   `json:"actor_user_id,omitempty"`
	ActorName      string    `json:"actor_name,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// BloodPickupSummary describes the live transport leg of a requisition.
type BloodPickupSummary struct {
	ID                      string     `json:"id"`
	Status                  string     `json:"status"`
	VehicleType             string     `json:"vehicle_type"`
	AmbulanceCode           string     `json:"ambulance_code,omitempty"`
	PlateNumber             string     `json:"plate_number,omitempty"`
	DriverName              string     `json:"driver_name,omitempty"`
	DriverPhone             string     `json:"driver_phone,omitempty"`
	PickupSiteName          string     `json:"pickup_site_name,omitempty"`
	DestinationFacilityName string     `json:"destination_facility_name,omitempty"`
	Notes                   string     `json:"notes,omitempty"`
	AssignedAt              time.Time  `json:"assigned_at"`
	CollectedAt             *time.Time `json:"collected_at,omitempty"`
	DeliveredAt             *time.Time `json:"delivered_at,omitempty"`
}

// BloodRequisitionTracking bundles history and transport for the detail view.
type BloodRequisitionTracking struct {
	Events []BloodRequisitionEvent `json:"events"`
	Pickup *BloodPickupSummary     `json:"pickup,omitempty"`
}
