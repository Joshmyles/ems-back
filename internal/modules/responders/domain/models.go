package domain

// Responder is a dispatch-facing view of an ambulance together with its active
// crew/driver, home station and current dispatch load. It is an aggregate
// derived from the fleet, reference and dispatch tables — there is no
// standalone responders table.
type Responder struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	AmbulanceUnit          string   `json:"ambulance_unit"`
	CrewType               string   `json:"crew_type"`
	VehicleType            string   `json:"vehicle_type"`
	District               string   `json:"district"`
	Base                   string   `json:"base"`
	Status                 string   `json:"status"`
	ETAMinutes             int      `json:"eta_minutes"`
	CurrentAssignmentCount int      `json:"current_assignment_count"`
	Capabilities           []string `json:"capabilities"`
	Phone                  string   `json:"phone"`
	NearestLandmark        string   `json:"nearest_landmark"`
}
