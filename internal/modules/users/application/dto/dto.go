package dto

import (
	platformdb "dispatch/internal/platform/db"
	"time"
)

type CreateUserRequest struct {
	StaffNo               string  `json:"staff_no"`
	Username              string  `json:"username" binding:"required"`
	FirstName             string  `json:"first_name" binding:"required"`
	LastName              string  `json:"last_name" binding:"required"`
	OtherName             string  `json:"other_name"`
	Gender                string  `json:"gender"`
	Phone                 string  `json:"phone"`
	Email                 string  `json:"email"`
	Password              string  `json:"password" binding:"omitempty,min=8"`
	PreferredLanguage     string  `json:"preferred_language"`
	Timezone              string  `json:"timezone"`
	Cadre                 string  `json:"cadre"`
	LicenseNumber         string  `json:"license_number"`
	Specialization        string  `json:"specialization"`
	DateOfBirth           *string `json:"date_of_birth"`
	NationalID            string  `json:"national_id"`
	AvatarURL             string  `json:"avatar_url"`
	EmergencyContactName  string  `json:"emergency_contact_name"`
	EmergencyContactPhone string  `json:"emergency_contact_phone"`
	Address               string  `json:"address"`
}

type UpdateUserRequest struct {
	StaffNo           *string `json:"staff_no"`
	FirstName         *string `json:"first_name"`
	LastName          *string `json:"last_name"`
	OtherName         *string `json:"other_name"`
	Gender            *string `json:"gender"`
	Phone             *string `json:"phone"`
	Email             *string `json:"email"`
	Status            *string `json:"status" enums:"ACTIVE,INACTIVE"`
	IsActive          *bool   `json:"is_active"`
	PreferredLanguage *string `json:"preferred_language"`
	Timezone          *string `json:"timezone"`
}

type UpdateUserProfileRequest struct {
	Cadre                 *string `json:"cadre"`
	LicenseNumber         *string `json:"license_number"`
	Specialization        *string `json:"specialization"`
	DateOfBirth           *string `json:"date_of_birth"`
	NationalID            *string `json:"national_id"`
	AvatarURL             *string `json:"avatar_url"`
	EmergencyContactName  *string `json:"emergency_contact_name"`
	EmergencyContactPhone *string `json:"emergency_contact_phone"`
	Address               *string `json:"address"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
	ResetByAdmin    bool   `json:"reset_by_admin"`
}

type AssignRoleRequest struct {
	RoleID     string  `json:"role_id" binding:"required"`
	ScopeType  string  `json:"scope_type" binding:"required"`
	ScopeID    *string `json:"scope_id"`
	AssignedBy *string `json:"assigned_by"`
}

type AssignUserRequest struct {
	DistrictID      *string `json:"district_id"`
	SubcountyID     *string `json:"subcounty_id"`
	FacilityID      *string `json:"facility_id"`
	AssignmentLevel string  `json:"assignment_level" binding:"required"`
	TeamName        string  `json:"team_name"`
	IsPrimary       bool    `json:"is_primary"`
	Active          bool    `json:"active"`
	StartDate       *string `json:"start_date"`
	EndDate         *string `json:"end_date"`
}

type AssignCapabilityRequest struct {
	CapabilityID string  `json:"capability_id" binding:"required"`
	LevelNo      *int    `json:"level_no"`
	IsActive     *bool   `json:"is_active"`
	ExpiresAt    *string `json:"expires_at"`
}

type ListUsersParams struct {
	Pagination platformdb.Pagination `json:"pagination"`
}

type UserRoleResponse struct {
	ID        string  `json:"id"`
	RoleID    string  `json:"role_id"`
	RoleCode  string  `json:"role_code"`
	RoleName  string  `json:"role_name"`
	ScopeType string  `json:"scope_type"`
	ScopeID   *string `json:"scope_id,omitempty"`
	Active    bool    `json:"active"`
}

type UserAssignmentResponse struct {
	ID              string     `json:"id"`
	DistrictID      *string    `json:"district_id,omitempty"`
	SubcountyID     *string    `json:"subcounty_id,omitempty"`
	FacilityID      *string    `json:"facility_id,omitempty"`
	AssignmentLevel string     `json:"assignment_level"`
	TeamName        string     `json:"team_name"`
	IsPrimary       bool       `json:"is_primary"`
	Active          bool       `json:"active"`
	StartDate       time.Time  `json:"start_date"`
	EndDate         *time.Time `json:"end_date,omitempty"`
}

type UserCapabilityResponse struct {
	ID             string     `json:"id"`
	CapabilityID   string     `json:"capability_id"`
	CapabilityCode string     `json:"capability_code"`
	CapabilityName string     `json:"capability_name"`
	LevelNo        *int       `json:"level_no,omitempty"`
	IsActive       bool       `json:"is_active"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

type UserDetailsResponse struct {
	User         any                      `json:"user"`
	Profile      any                      `json:"profile"`
	Roles        []UserRoleResponse       `json:"roles"`
	Assignments  []UserAssignmentResponse `json:"assignments"`
	Capabilities []UserCapabilityResponse `json:"capabilities"`
}
