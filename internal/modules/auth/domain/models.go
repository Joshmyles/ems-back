package domain

import "time"

type UserSession struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	RefreshTokenID string     `json:"refresh_token_id"`
	AccessTokenID  string     `json:"access_token_id"`
	DeviceID       string     `json:"device_id"`
	DeviceName     string     `json:"device_name"`
	UserAgent      string     `json:"user_agent"`
	IPAddress      string     `json:"ip_address"`
	LastActivityAt time.Time  `json:"last_activity_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	RevokeReason   string     `json:"revoke_reason,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type AuthUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email"`
	Phone        string    `json:"phone"`
	PasswordHash string    `json:"-"`
	Status       string    `json:"status"`
	IsActive     bool      `json:"is_active"`
	Roles        []string  `json:"roles"`
	LastLoginAt  time.Time `json:"last_login_at"`
}
