package application

import (
	"context"
	"time"

	authdomain "dispatch/internal/modules/auth/domain"

	deviceapp "dispatch/internal/modules/device_tokens/application"
	devicedomain "dispatch/internal/modules/device_tokens/domain"
)

type Repository interface {
	GetUserForLogin(ctx context.Context, username string) (authdomain.AuthUser, error)
	CreateSession(ctx context.Context, session authdomain.UserSession) error
	GetSessionByRefreshTokenID(ctx context.Context, refreshTokenID string) (authdomain.UserSession, error)
	TouchSession(ctx context.Context, sessionID string, at time.Time, newAccessTokenID string) error
	RevokeSession(ctx context.Context, sessionID string, reason string) error
	RevokeOwnSession(ctx context.Context, userID, sessionID string) (string, time.Time, error)
	RevokeAllUserSessions(ctx context.Context, userID string, reason string) error
	ListActiveSessions(ctx context.Context, userID string) ([]authdomain.UserSession, error)
	UpdateLastLogin(ctx context.Context, userID string, at time.Time) error
	IncrementFailedLogin(ctx context.Context, userID string) error
	ResetFailedLogin(ctx context.Context, userID string) error
}

type DeviceTokenService interface {
	Register(ctx context.Context, req deviceapp.RegisterDeviceTokenRequest) (devicedomain.DeviceToken, error)
}
