package infrastructure

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	authdomain "dispatch/internal/modules/auth/domain"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetUserForLogin(ctx context.Context, username string) (authdomain.AuthUser, error) {
	query := `
	SELECT u.id, u.username, COALESCE(u.email,''), COALESCE(u.phone,''), u.password_hash,
	       u.status, u.is_active, COALESCE(u.last_login_at, now())
	FROM users u
	WHERE (u.username = $1 OR u.email = $1 OR u.phone = $1)
	  AND u.deleted_at IS NULL
	LIMIT 1`

	var user authdomain.AuthUser
	if err := r.db.QueryRow(ctx, query, username).Scan(
		&user.ID,
		&user.Username,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&user.Status,
		&user.IsActive,
		&user.LastLoginAt,
	); err != nil {
		return authdomain.AuthUser{}, err
	}

	rolesQuery := `
	SELECT r.code
	FROM user_roles ur
	JOIN roles r ON r.id = ur.role_id
	WHERE ur.user_id = $1 AND ur.active = TRUE
	ORDER BY r.code`
	rows, err := r.db.Query(ctx, rolesQuery, user.ID)
	if err != nil {
		return authdomain.AuthUser{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return authdomain.AuthUser{}, err
		}
		user.Roles = append(user.Roles, role)
	}
	return user, rows.Err()
}

func (r *Repository) CreateSession(ctx context.Context, session authdomain.UserSession) error {
	_, err := r.db.Exec(ctx, `
	INSERT INTO auth_sessions (
		id, user_id, refresh_token_id, access_token_id, device_id, device_name,
		user_agent, ip_address, last_activity_at, expires_at, created_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	`, session.ID, session.UserID, session.RefreshTokenID, session.AccessTokenID, session.DeviceID, session.DeviceName,
		session.UserAgent, session.IPAddress, session.LastActivityAt, session.ExpiresAt, session.CreatedAt)
	return err
}

func (r *Repository) GetSessionByRefreshTokenID(ctx context.Context, refreshTokenID string) (authdomain.UserSession, error) {
	var s authdomain.UserSession
	err := r.db.QueryRow(ctx, `
	SELECT id, user_id, refresh_token_id, access_token_id, COALESCE(device_id,''), COALESCE(device_name,''),
	       COALESCE(user_agent,''), COALESCE(ip_address,''), last_activity_at, expires_at, revoked_at,
	       COALESCE(revoke_reason,''), created_at
	FROM auth_sessions
	WHERE refresh_token_id = $1
	LIMIT 1`, refreshTokenID).Scan(
		&s.ID, &s.UserID, &s.RefreshTokenID, &s.AccessTokenID, &s.DeviceID, &s.DeviceName,
		&s.UserAgent, &s.IPAddress, &s.LastActivityAt, &s.ExpiresAt, &s.RevokedAt,
		&s.RevokeReason, &s.CreatedAt,
	)
	return s, err
}

func (r *Repository) TouchSession(ctx context.Context, sessionID string, at time.Time, newAccessTokenID string) error {
	_, err := r.db.Exec(ctx, `
	UPDATE auth_sessions
	SET last_activity_at = $2,
	    access_token_id = $3
	WHERE id = $1`, sessionID, at, newAccessTokenID)
	return err
}

func (r *Repository) RevokeSession(ctx context.Context, sessionID string, reason string) error {
	_, err := r.db.Exec(ctx, `
	UPDATE auth_sessions
	SET revoked_at = now(), revoke_reason = $2
	WHERE id = $1 AND revoked_at IS NULL`, sessionID, reason)
	return err
}

// RevokeOwnSession revokes a single session but only if it belongs to the
// given user. It returns the session's refresh-token id and expiry so the
// caller can also blacklist the refresh token in the cache. pgx.ErrNoRows means
// the session does not exist, is not owned by the user, or is already revoked.
func (r *Repository) RevokeOwnSession(ctx context.Context, userID, sessionID string) (string, time.Time, error) {
	var refreshTokenID string
	var expiresAt time.Time
	err := r.db.QueryRow(ctx, `
	UPDATE auth_sessions
	SET revoked_at = now(), revoke_reason = 'user_revoked'
	WHERE id = $1 AND user_id = $2 AND revoked_at IS NULL
	RETURNING refresh_token_id, expires_at`, sessionID, userID).Scan(&refreshTokenID, &expiresAt)
	return refreshTokenID, expiresAt, err
}

func (r *Repository) RevokeAllUserSessions(ctx context.Context, userID string, reason string) error {
	_, err := r.db.Exec(ctx, `
	UPDATE auth_sessions
	SET revoked_at = now(), revoke_reason = $2
	WHERE user_id = $1 AND revoked_at IS NULL`, userID, reason)
	return err
}

func (r *Repository) ListActiveSessions(ctx context.Context, userID string) ([]authdomain.UserSession, error) {
	rows, err := r.db.Query(ctx, `
	SELECT id, user_id, refresh_token_id, access_token_id, COALESCE(device_id,''), COALESCE(device_name,''),
	       COALESCE(user_agent,''), COALESCE(ip_address,''), last_activity_at, expires_at, revoked_at,
	       COALESCE(revoke_reason,''), created_at
	FROM auth_sessions
	WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > now()
	ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []authdomain.UserSession
	for rows.Next() {
		var s authdomain.UserSession
		if err := rows.Scan(
			&s.ID, &s.UserID, &s.RefreshTokenID, &s.AccessTokenID, &s.DeviceID, &s.DeviceName,
			&s.UserAgent, &s.IPAddress, &s.LastActivityAt, &s.ExpiresAt, &s.RevokedAt,
			&s.RevokeReason, &s.CreatedAt,
		); err != nil {
			return nil, err
		}
		sessions = append(sessions, s)
	}
	return sessions, rows.Err()
}

func (r *Repository) UpdateLastLogin(ctx context.Context, userID string, at time.Time) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = $2, updated_at = $2 WHERE id = $1`, userID, at)
	return err
}

func (r *Repository) IncrementFailedLogin(ctx context.Context, userID string) error {
	// Accounts are no longer auto-locked; the counter is kept for visibility
	// only. Access is controlled by the account's active/inactive state.
	_, err := r.db.Exec(ctx, `
	UPDATE users
	SET failed_login_attempts = failed_login_attempts + 1,
	    updated_at = now()
	WHERE id = $1`, userID)
	return err
}

func (r *Repository) ResetFailedLogin(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET failed_login_attempts = 0, updated_at = now() WHERE id = $1`, userID)
	return err
}

var errNoRows = errors.New("not found")

func isNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
