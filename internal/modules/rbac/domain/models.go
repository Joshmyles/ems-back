package domain

import "time"

type PermissionGrant struct {
	UserID    string  `json:"user_id"`
	RoleCode  string  `json:"role_code"`
	PermCode  string  `json:"perm_code"`
	ScopeType string  `json:"scope_type"`
	ScopeID   *string `json:"scope_id,omitempty"`
}

// Permission is a single entry in the permission catalogue.
type Permission struct {
	ID          string `json:"id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Module      string `json:"module"`
	Description string `json:"description"`
}

// Role is a named bundle of permissions.
type Role struct {
	ID              string    `json:"id"`
	Code            string    `json:"code"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	IsSystem        bool      `json:"is_system"`
	PermissionCount int       `json:"permission_count"`
	MemberCount     int       `json:"member_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// RoleDetail is a role plus its resolved permission set.
type RoleDetail struct {
	Role
	Permissions []Permission `json:"permissions"`
}
