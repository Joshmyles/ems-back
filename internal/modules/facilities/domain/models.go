package domain

type Region struct {
	RegionUID string `json:"region_uid"`
	Region    string `json:"region"`
}

type District struct {
	DistrictUID string `json:"district_uid"`
	RegionUID   string `json:"region_uid"`
	District    string `json:"district"`
}

type Subcounty struct {
	SubcountyUID string `json:"subcounty_uid"`
	DistrictUID  string `json:"district_uid"`
	Subcounty    string `json:"subcounty"`
}

type Facility struct {
	FacilityUID  string `json:"facility_uid"`
	SubcountyUID string `json:"subcounty_uid"`
	Facility     string `json:"facility"`
	Level        string `json:"level"`
	Ownership    string `json:"ownership"`

	RegionUID   string `json:"region_uid"`
	DistrictUID string `json:"district_uid"`
	Region      string `json:"region"`
	District    string `json:"district"`
	Subcounty   string `json:"subcounty"`

	FocalPerson *FocalPerson `json:"focal_person,omitempty"`
}

type FocalPerson struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
}
