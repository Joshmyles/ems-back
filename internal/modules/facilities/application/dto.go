package application

type CreateFacilityRequest struct {
	FacilityUID  string `json:"facility_uid" binding:"required"`
	SubcountyUID string `json:"subcounty_uid" binding:"required"`
	Facility     string `json:"facility" binding:"required"`
	Level        string `json:"level"`
	Ownership    string `json:"ownership"`
}

type UpdateFacilityRequest struct {
	SubcountyUID *string `json:"subcounty_uid,omitempty"`
	Facility     *string `json:"facility,omitempty"`
	Level        *string `json:"level,omitempty"`
	Ownership    *string `json:"ownership,omitempty"`
}

type SetFocalPersonRequest struct {
	UserID string `json:"user_id" binding:"required,uuid"`
}
