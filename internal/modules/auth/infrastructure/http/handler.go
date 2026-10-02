package http

import (
	"errors"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	authapp "dispatch/internal/modules/auth/application"
	dto "dispatch/internal/modules/auth/application/dto"
	"dispatch/internal/platform/httpx"
	"dispatch/internal/shared/types"
)

type Handler struct {
	service *authapp.Service
	audit   types.AuditRecorder
}

func NewHandler(service *authapp.Service, audit types.AuditRecorder) *Handler {
	if audit == nil {
		audit = types.NoopAuditRecorder{}
	}
	return &Handler{service: service, audit: audit}
}

// Login godoc
//
//	@Summary		Login
//	@Description	Authenticates a user and returns access and refresh tokens
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		dto.LoginRequest	true	"Login payload"
//	@Success		200		{object}	dto.AuthResponse
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/auth/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	deviceID := c.GetString("device_id")
	deviceName := c.GetString("device_name")
	ip := clientIP(c.ClientIP())
	out, err := h.service.Login(c.Request.Context(), req, deviceID, deviceName, ip, c.Request.UserAgent())
	if err != nil {
		// Record the failed attempt. The actor is unknown (bad credentials),
		// so only the attempted username is captured.
		h.audit.Record(c.Request.Context(), types.AuditEntry{
			ActorUsername: req.Username,
			Action:        types.AuditAuthLoginFailed,
			EntityType:    "auth",
			Status:        types.AuditStatusFailure,
			Description:   "Failed login for '" + req.Username + "': " + err.Error(),
			IPAddress:     ip,
			UserAgent:     c.Request.UserAgent(),
		})
		switch {
		case errors.Is(err, authapp.ErrInvalidCredentials):
			httpx.Error(c, http.StatusUnauthorized, "invalid credentials")
		case errors.Is(err, authapp.ErrInactiveUser):
			httpx.Error(c, http.StatusForbidden, "account is deactivated")
		default:
			httpx.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   out.User.ID,
		ActorUsername: out.User.Username,
		ActorRoles:    out.User.Roles,
		Action:        types.AuditAuthLogin,
		EntityType:    "auth",
		EntityID:      out.User.ID,
		Status:        types.AuditStatusSuccess,
		Description:   "User '" + out.User.Username + "' logged in",
		IPAddress:     ip,
		UserAgent:     c.Request.UserAgent(),
		Metadata:      map[string]any{"device_name": deviceName},
	})
	httpx.OK(c, out)
}

// Refresh godoc
//
//	@Summary		Refresh tokens
//	@Description	Refreshes the access token using the refresh token
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		dto.RefreshRequest	true	"Refresh payload"
//	@Success		200		{object}	dto.AuthResponse
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/auth/refresh [post]
func (h *Handler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "invalid refresh token")
		return
	}
	httpx.OK(c, out)
}

// Logout godoc
//
//	@Summary		Logout
//	@Description	Revokes current session or all sessions
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dto.LogoutRequest	true	"Logout payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		401		{object}	map[string]interface{}
//	@Router			/auth/logout [post]
func (h *Handler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := c.GetString("user_id")
	var err error
	action := types.AuditAuthLogout
	desc := "User logged out"
	if req.LogoutAll {
		action = types.AuditAuthLogoutAll
		desc = "User logged out of all sessions"
		err = h.service.LogoutAll(c.Request.Context(), userID)
	} else {
		err = h.service.Logout(c.Request.Context(), req.RefreshToken)
	}
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, err.Error())
		return
	}
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   userID,
		ActorUsername: c.GetString("username"),
		Action:        action,
		EntityType:    "auth",
		EntityID:      userID,
		Status:        types.AuditStatusSuccess,
		Description:   desc,
		IPAddress:     clientIP(c.ClientIP()),
		UserAgent:     c.Request.UserAgent(),
	})
	httpx.OK(c, gin.H{"message": "logged out"})
}

// Sessions godoc
//
//	@Summary		List sessions
//	@Description	Lists active user sessions
//	@Tags			Auth
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]interface{}
//	@Router			/auth/sessions [get]
func (h *Handler) Sessions(c *gin.Context) {
	userID := c.GetString("user_id")
	items, err := h.service.Sessions(c.Request.Context(), userID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, items)
}

// RevokeSession godoc
//
//	@Summary		Revoke a session
//	@Description	Revokes one of the authenticated user's own active sessions by id
//	@Tags			Auth
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Session ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		401	{object}	map[string]interface{}
//	@Router			/auth/sessions/{id} [delete]
func (h *Handler) RevokeSession(c *gin.Context) {
	userID := c.GetString("user_id")
	sessionID := c.Param("id")
	if err := h.service.RevokeSession(c.Request.Context(), userID, sessionID); err != nil {
		httpx.Error(c, http.StatusUnauthorized, "session not found")
		return
	}
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   userID,
		ActorUsername: c.GetString("username"),
		Action:        types.AuditAuthLogout,
		EntityType:    "auth_session",
		EntityID:      sessionID,
		Status:        types.AuditStatusSuccess,
		Description:   "Revoked a session",
		IPAddress:     clientIP(c.ClientIP()),
		UserAgent:     c.Request.UserAgent(),
	})
	httpx.OK(c, gin.H{"message": "session revoked"})
}

func clientIP(raw string) string {
	ip := strings.TrimSpace(raw)
	if host, _, err := net.SplitHostPort(ip); err == nil {
		return host
	}
	return ip
}
