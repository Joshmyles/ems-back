package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"dispatch/internal/modules/auth/application/dto"
	authdomain "dispatch/internal/modules/auth/domain"
	deviceapp "dispatch/internal/modules/device_tokens/application"
	"dispatch/internal/modules/rbac/application"

	platformauth "dispatch/internal/platform/auth"
	"dispatch/internal/platform/events"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInactiveUser       = errors.New("user inactive")
	ErrInvalidSession     = errors.New("invalid session")
	ErrExpiredSession     = errors.New("session expired")
)

type Service struct {
	repo         Repository
	rbacRepo     application.Repository
	jwt          *platformauth.JWTManager
	redis        *redis.Client
	log          *zap.Logger
	accessTTL    time.Duration
	refreshTTL   time.Duration
	bus          events.Publisher
	deviceTokens DeviceTokenService
}

func NewService(
	repo Repository,
	rbacRepo application.Repository,
	jwt *platformauth.JWTManager,
	redisClient *redis.Client,
	log *zap.Logger,
	accessTTL, refreshTTL time.Duration,
	bus events.Publisher,
	deviceTokens DeviceTokenService,
) *Service {
	return &Service{
		repo:         repo,
		rbacRepo:     rbacRepo,
		jwt:          jwt,
		redis:        redisClient,
		log:          log,
		accessTTL:    accessTTL,
		refreshTTL:   refreshTTL,
		bus:          bus,
		deviceTokens: deviceTokens,
	}
}

func (s *Service) Login(ctx context.Context, req dto.LoginRequest, deviceID, deviceName, ipAddress, userAgent string) (dto.AuthResponse, error) {
	user, err := s.repo.GetUserForLogin(ctx, req.Username)
	if err != nil {
		return dto.AuthResponse{}, ErrInvalidCredentials
	}
	if !user.IsActive || user.Status != "ACTIVE" {
		return dto.AuthResponse{}, ErrInactiveUser
	}
	if err := platformauth.CheckPassword(user.PasswordHash, req.Password); err != nil {
		_ = s.repo.IncrementFailedLogin(ctx, user.ID)
		return dto.AuthResponse{}, ErrInvalidCredentials
	}

	now := time.Now().UTC()
	_ = s.repo.ResetFailedLogin(ctx, user.ID)
	_ = s.repo.UpdateLastLogin(ctx, user.ID, now)

	accessJTI := uuid.NewString()
	refreshJTI := uuid.NewString()
	sessionID := uuid.NewString()

	accessToken, err := s.jwt.GenerateAccessTokenWithJTI(user.ID, user.Username, user.Roles, accessJTI)
	if err != nil {
		return dto.AuthResponse{}, err
	}
	refreshToken, err := s.jwt.GenerateRefreshToken(user.ID, user.Username, refreshJTI)
	if err != nil {
		return dto.AuthResponse{}, err
	}

	session := authdomain.UserSession{
		ID:             sessionID,
		UserID:         user.ID,
		RefreshTokenID: refreshJTI,
		AccessTokenID:  accessJTI,
		DeviceID:       deviceID,
		DeviceName:     deviceName,
		UserAgent:      userAgent,
		IPAddress:      ipAddress,
		LastActivityAt: now,
		ExpiresAt:      now.Add(s.refreshTTL),
		CreatedAt:      now,
	}
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return dto.AuthResponse{}, err
	}
	if s.redis != nil {
		if err := s.cacheSession(ctx, session); err != nil {
			s.log.Warn("cache session", zap.Error(err))
		}
	}

	if s.deviceTokens != nil && strings.TrimSpace(req.PushToken) != "" {
		_, err := s.deviceTokens.Register(ctx, deviceapp.RegisterDeviceTokenRequest{
			UserID:    user.ID,
			DeviceID:  strings.TrimSpace(req.DeviceID),
			Platform:  strings.ToUpper(strings.TrimSpace(req.Platform)),
			PushToken: strings.TrimSpace(req.PushToken),
		})
		if err != nil && s.log != nil {
			s.log.Warn("failed to register device token on login",
				zap.String("user_id", user.ID),
				zap.Error(err),
			)
		}
	}

	perms, err := s.rbacRepo.ListPermissionGrants(ctx, user.ID)
	if err != nil {
		return dto.AuthResponse{}, err
	}

	permissionCodes := make([]string, 0, len(perms))
	seen := make(map[string]struct{})

	for _, p := range perms {
		if _, exists := seen[p.PermCode]; exists {
			continue
		}
		seen[p.PermCode] = struct{}{}
		permissionCodes = append(permissionCodes, p.PermCode)
	}

	return dto.AuthResponse{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresIn:  int64(s.accessTTL.Seconds()),
		RefreshTokenExpiresIn: int64(s.refreshTTL.Seconds()),
		Session: dto.SessionResponse{
			ID:             session.ID,
			DeviceID:       session.DeviceID,
			DeviceName:     session.DeviceName,
			IPAddress:      session.IPAddress,
			UserAgent:      session.UserAgent,
			LastActivityAt: session.LastActivityAt.Format(time.RFC3339),
			ExpiresAt:      session.ExpiresAt.Format(time.RFC3339),
		},
		User: dto.AuthUserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Phone:    user.Phone,
			Status:   user.Status,
			Roles:    user.Roles,
		},
		Permissions: permissionCodes,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, refreshToken string) (dto.AuthResponse, error) {
	claims, err := s.jwt.ParseRefreshToken(refreshToken)
	if err != nil {
		return dto.AuthResponse{}, ErrInvalidSession
	}
	if s.isRefreshTokenRevoked(ctx, claims.ID) {
		return dto.AuthResponse{}, ErrInvalidSession
	}

	session, err := s.getSession(ctx, claims.ID)
	if err != nil {
		return dto.AuthResponse{}, ErrInvalidSession
	}
	if session.RevokedAt != nil {
		return dto.AuthResponse{}, ErrInvalidSession
	}
	if time.Now().UTC().After(session.ExpiresAt) {
		return dto.AuthResponse{}, ErrExpiredSession
	}

	user, err := s.repo.GetUserForLogin(ctx, claims.Username)
	if err != nil {
		return dto.AuthResponse{}, ErrInvalidSession
	}
	if !user.IsActive {
		return dto.AuthResponse{}, ErrInvalidSession
	}

	newAccessJTI := uuid.NewString()
	accessToken, err := s.jwt.GenerateAccessTokenWithJTI(user.ID, user.Username, user.Roles, newAccessJTI)
	if err != nil {
		return dto.AuthResponse{}, err
	}

	now := time.Now().UTC()
	if err := s.repo.TouchSession(ctx, session.ID, now, newAccessJTI); err != nil {
		return dto.AuthResponse{}, err
	}
	session.AccessTokenID = newAccessJTI
	session.LastActivityAt = now
	_ = s.cacheSession(ctx, session)

	return dto.AuthResponse{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		AccessTokenExpiresIn:  int64(s.accessTTL.Seconds()),
		RefreshTokenExpiresIn: int64(time.Until(session.ExpiresAt).Seconds()),
		Session: dto.SessionResponse{
			ID:             session.ID,
			DeviceID:       session.DeviceID,
			DeviceName:     session.DeviceName,
			IPAddress:      session.IPAddress,
			UserAgent:      session.UserAgent,
			LastActivityAt: session.LastActivityAt.Format(time.RFC3339),
			ExpiresAt:      session.ExpiresAt.Format(time.RFC3339),
		},
		User: dto.AuthUserResponse{
			ID:       user.ID,
			Username: user.Username,
			Email:    user.Email,
			Phone:    user.Phone,
			Status:   user.Status,
			Roles:    user.Roles,
		},
	}, nil
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	if refreshToken == "" {
		return ErrInvalidSession
	}
	claims, err := s.jwt.ParseRefreshToken(refreshToken)
	if err != nil {
		return ErrInvalidSession
	}
	session, err := s.getSession(ctx, claims.ID)
	if err != nil {
		return ErrInvalidSession
	}
	if err := s.repo.RevokeSession(ctx, session.ID, "logout"); err != nil {
		return err
	}
	if s.redis != nil {
		_ = s.revokeRefreshToken(ctx, session.RefreshTokenID, time.Until(session.ExpiresAt))
		_ = s.redis.Del(ctx, s.sessionCacheKey(session.RefreshTokenID)).Err()
	}
	return nil
}

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	sessions, err := s.repo.ListActiveSessions(ctx, userID)
	if err != nil {
		return err
	}
	if err := s.repo.RevokeAllUserSessions(ctx, userID, "logout_all"); err != nil {
		return err
	}
	if s.redis != nil {
		for _, session := range sessions {
			_ = s.revokeRefreshToken(ctx, session.RefreshTokenID, time.Until(session.ExpiresAt))
			_ = s.redis.Del(ctx, s.sessionCacheKey(session.RefreshTokenID)).Err()
		}
	}
	return nil
}

func (s *Service) Sessions(ctx context.Context, userID string) ([]authdomain.UserSession, error) {
	return s.repo.ListActiveSessions(ctx, userID)
}

// RevokeSession revokes one of the user's own sessions by id. It fails with
// ErrInvalidSession when the session does not exist or is not owned by the user.
func (s *Service) RevokeSession(ctx context.Context, userID, sessionID string) error {
	refreshJTI, expiresAt, err := s.repo.RevokeOwnSession(ctx, userID, sessionID)
	if err != nil {
		return ErrInvalidSession
	}
	if s.redis != nil {
		_ = s.revokeRefreshToken(ctx, refreshJTI, time.Until(expiresAt))
		_ = s.redis.Del(ctx, s.sessionCacheKey(refreshJTI)).Err()
	}
	return nil
}

func (s *Service) cacheSession(ctx context.Context, session authdomain.UserSession) error {
	if s.redis == nil {
		return nil
	}
	key := s.sessionCacheKey(session.RefreshTokenID)
	value := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s", session.ID, session.UserID, session.AccessTokenID, session.DeviceID, session.DeviceName, session.IPAddress, session.ExpiresAt.Format(time.RFC3339))
	return s.redis.Set(ctx, key, value, time.Until(session.ExpiresAt)).Err()
}

func (s *Service) sessionCacheKey(refreshJTI string) string {
	return "auth:session:refresh:" + refreshJTI
}

func (s *Service) revokeRefreshToken(ctx context.Context, refreshJTI string, ttl time.Duration) error {
	if s.redis == nil {
		return nil
	}
	if ttl < time.Minute {
		ttl = time.Minute
	}
	return s.redis.Set(ctx, "auth:revoked:refresh:"+refreshJTI, "1", ttl).Err()
}

func (s *Service) isRefreshTokenRevoked(ctx context.Context, refreshJTI string) bool {
	if s.redis == nil {
		return false
	}
	ok, err := s.redis.Exists(ctx, "auth:revoked:refresh:"+refreshJTI).Result()
	if err != nil {
		s.log.Warn("check revoked refresh token", zap.Error(err))
		return false
	}
	return ok > 0
}

func (s *Service) getSession(ctx context.Context, refreshJTI string) (authdomain.UserSession, error) {
	session, err := s.repo.GetSessionByRefreshTokenID(ctx, refreshJTI)
	if err != nil {
		return authdomain.UserSession{}, err
	}
	return session, nil
}
