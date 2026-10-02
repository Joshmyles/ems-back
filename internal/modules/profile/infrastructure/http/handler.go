package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
	userapp "dispatch/internal/modules/users/application"
	userdto "dispatch/internal/modules/users/application/dto"
	"dispatch/internal/platform/httpx"
	"dispatch/internal/shared/types"
)

type Handler struct {
	users *userapp.Service
	rbac  *rbacapp.Service
	audit types.AuditRecorder
}

func NewHandler(users *userapp.Service, rbac *rbacapp.Service, audit types.AuditRecorder) *Handler {
	if audit == nil {
		audit = types.NoopAuditRecorder{}
	}
	return &Handler{users: users, rbac: rbac, audit: audit}
}

// Me godoc
//
//	@Summary		Get my profile
//	@Description	Returns the authenticated user's account, profile, roles and effective permissions
//	@Tags			Profile
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]interface{}
//	@Router			/me [get]
func (h *Handler) Me(c *gin.Context) {
	userID := c.GetString("user_id")
	details, err := h.users.GetDetails(c.Request.Context(), userID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	permissions := []string{}
	if h.rbac != nil {
		grants, err := h.rbac.ListPermissionGrants(c.Request.Context(), userID)
		if err == nil {
			seen := map[string]struct{}{}
			for _, g := range grants {
				if _, ok := seen[g.PermCode]; ok {
					continue
				}
				seen[g.PermCode] = struct{}{}
				permissions = append(permissions, g.PermCode)
			}
		}
	}

	httpx.OK(c, gin.H{
		"user":         details.User,
		"profile":      details.Profile,
		"roles":        details.Roles,
		"assignments":  details.Assignments,
		"capabilities": details.Capabilities,
		"permissions":  permissions,
	})
}

type updateMeRequest struct {
	FirstName *string `json:"first_name"`
	LastName  *string `json:"last_name"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
}

// UpdateMe godoc
//
//	@Summary		Update my profile
//	@Description	Updates the authenticated user's own basic details (name, email, phone)
//	@Tags			Profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		updateMeRequest	true	"Profile fields"
//	@Success		200		{object}	map[string]interface{}
//	@Router			/me [patch]
func (h *Handler) UpdateMe(c *gin.Context) {
	var req updateMeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := c.GetString("user_id")

	// Only safe self-service fields are forwarded — a user can never change
	// their own status, active flag or roles here.
	user, err := h.users.Update(c.Request.Context(), userID, userdto.UpdateUserRequest{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Phone:     req.Phone,
	})
	if err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.record(c, types.AuditUserProfileUpdate, userID, "Updated own profile")
	httpx.OK(c, user)
}

type changeMyPasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// ChangeMyPassword godoc
//
//	@Summary		Change my password
//	@Description	Changes the authenticated user's own password after verifying the current one
//	@Tags			Profile
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		changeMyPasswordRequest	true	"Password change"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Router			/me/change-password [post]
func (h *Handler) ChangeMyPassword(c *gin.Context) {
	var req changeMyPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	userID := c.GetString("user_id")

	err := h.users.ChangePassword(c.Request.Context(), userID, userdto.ChangePasswordRequest{
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	}, userID, false)
	if err != nil {
		if errors.Is(err, userapp.ErrForbidden) {
			httpx.Error(c, http.StatusForbidden, "not permitted")
			return
		}
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	h.record(c, types.AuditUserPasswordChange, userID, "Changed own password")
	httpx.OK(c, gin.H{"message": "password changed successfully"})
}

func (h *Handler) record(c *gin.Context, action, entityID, desc string) {
	var roles []string
	if v, ok := c.Get("roles"); ok {
		roles, _ = v.([]string)
	}
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   c.GetString("user_id"),
		ActorUsername: c.GetString("username"),
		ActorRoles:    roles,
		Action:        action,
		EntityType:    "user",
		EntityID:      entityID,
		Status:        types.AuditStatusSuccess,
		Description:   desc,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
}
