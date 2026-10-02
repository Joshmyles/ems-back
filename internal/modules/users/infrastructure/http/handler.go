package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
	userSv "dispatch/internal/modules/users/application"
	"dispatch/internal/modules/users/application/dto"
	userdto "dispatch/internal/modules/users/application/dto"
	"dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
	"dispatch/internal/shared/types"
)

type Handler struct {
	service *userSv.Service
	rbac    *rbacapp.Service
	audit   types.AuditRecorder
}

func NewHandler(service *userSv.Service, rbac *rbacapp.Service, audit types.AuditRecorder) *Handler {
	if audit == nil {
		audit = types.NoopAuditRecorder{}
	}
	return &Handler{service: service, rbac: rbac, audit: audit}
}

// Create godoc
//
//	@Summary		Create user
//	@Description	Creates a new system user
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		userdto.CreateUserRequest	true	"Create user payload"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users [post]
func (h *Handler) Create(c *gin.Context) {
	var req userdto.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	user, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.rec(c, types.AuditUserCreate, "user", user.ID, "Created user "+req.Username, nil,
		map[string]any{"username": req.Username, "email": req.Email, "phone": req.Phone})
	httpx.Created(c, user)
}

// List godoc
//
//	@Summary		List users
//	@Description	Returns paginated users with search, sorting, and filters
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page				query		int		false	"Page number"	default(1)
//	@Param			page_size			query		int		false	"Page size"		default(20)
//	@Param			search				query		string	false	"Search term"
//	@Param			sort_by				query		string	false	"Sort field"	Enums(created_at,username,first_name,last_name,status)
//	@Param			sort_order			query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Param			filter[status]		query		string	false	"Filter by status"
//	@Param			filter[is_active]	query		string	false	"Filter by active flag"
//	@Success		200					{object}	map[string]interface{}
//	@Failure		500					{object}	map[string]interface{}
//	@Router			/users [get]
func (h *Handler) List(c *gin.Context) {
	params := dto.ListUsersParams{
		Pagination: db.ParsePagination(
			c.Request.URL.Query(),
			map[string]string{
				"created_at": "u.created_at",
				"username":   "u.username",
				"first_name": "u.first_name",
				"last_name":  "u.last_name",
				"status":     "u.status",
			},
			map[string]struct{}{
				"status":    {},
				"is_active": {},
				"role":      {},
			},
		),
	}

	result, err := h.service.List(c.Request.Context(), params)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, result)
}

// GetByID godoc
//
//	@Summary		Get user by ID
//	@Description	Returns a user by their ID
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/users/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	user, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, userSv.ErrUserNotFound) {
			httpx.Error(c, http.StatusNotFound, err.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, user)
}

// Update godoc
//
//	@Summary		Update user
//	@Description	Updates user details. All fields are optional.
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"User ID"
//	@Param			payload	body		userdto.UpdateUserRequest	true	"Update user payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	var req dto.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	before, _ := h.service.GetByID(c.Request.Context(), c.Param("id"))
	user, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
	if err != nil {
		if errors.Is(err, userSv.ErrUserNotFound) {
			httpx.Error(c, http.StatusNotFound, err.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.rec(c, types.AuditUserUpdate, "user", c.Param("id"), "Updated user", before, user)
	httpx.OK(c, user)
}

// Delete godoc
//
//	@Summary		Delete user
//	@Description	Soft deletes a user by their ID
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/users/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	before, _ := h.service.GetByID(c.Request.Context(), c.Param("id"))
	err := h.service.Delete(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, userSv.ErrUserNotFound) {
			httpx.Error(c, http.StatusNotFound, err.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.rec(c, types.AuditUserDelete, "user", c.Param("id"), "Deleted user", before, nil)
	httpx.OK(c, gin.H{"message": "user deleted"})
}

// Activate godoc
//
//	@Summary		Activate user
//	@Description	Re-enables a deactivated user account and clears the failed-login counter
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/users/{id}/activate [post]
func (h *Handler) Activate(c *gin.Context) {
	user, err := h.service.Activate(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, userSv.ErrUserNotFound) {
			httpx.Error(c, http.StatusNotFound, err.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.rec(c, types.AuditUserUpdate, "user", c.Param("id"), "Activated user account", nil,
		map[string]any{"is_active": true, "status": "ACTIVE"})
	httpx.OK(c, user)
}

// Deactivate godoc
//
//	@Summary		Deactivate user
//	@Description	Disables a user account and revokes all of its active sessions
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/users/{id}/deactivate [post]
func (h *Handler) Deactivate(c *gin.Context) {
	user, err := h.service.Deactivate(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, userSv.ErrUserNotFound) {
			httpx.Error(c, http.StatusNotFound, err.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.rec(c, types.AuditUserUpdate, "user", c.Param("id"), "Deactivated user account", nil,
		map[string]any{"is_active": false, "status": "INACTIVE"})
	httpx.OK(c, user)
}

// ChangePassword godoc
//
//	@Summary		Change user password
//	@Description	Self-service password change (verifies current password), or an
//	@Description	admin reset for another user (requires users.update).
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"User ID"
//	@Param			payload	body		userdto.ChangePasswordRequest	true	"Change password payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		403		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/change-password [post]
func (h *Handler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	targetID := c.Param("id")
	callerID := c.GetString("user_id")

	// An admin reset (reset_by_admin, or acting on another user) requires the
	// users.update privilege. Self-service with the current password does not.
	canAdminReset := h.hasPermission(c, "users.update")

	if err := h.service.ChangePassword(c.Request.Context(), targetID, req, callerID, canAdminReset); err != nil {
		switch {
		case errors.Is(err, userSv.ErrForbidden):
			h.recDenied(c, types.AuditUserPasswordReset, "user", targetID,
				"Denied password reset (insufficient privilege)")
			httpx.Error(c, http.StatusForbidden, "not permitted to reset this password")
		case errors.Is(err, userSv.ErrUserNotFound):
			httpx.Error(c, http.StatusNotFound, err.Error())
		default:
			httpx.Error(c, http.StatusBadRequest, err.Error())
		}
		return
	}
	action := types.AuditUserPasswordChange
	desc := "Changed own password"
	if targetID != callerID || req.ResetByAdmin {
		action = types.AuditUserPasswordReset
		desc = "Reset password for another user"
	}
	h.rec(c, action, "user", targetID, desc, nil, nil)
	httpx.OK(c, gin.H{"message": "password changed successfully"})
}

// AssignRole godoc
//
//	@Summary		Assign role to user
//	@Description	Assigns a role to a user with optional scope. Guarded against
//	@Description	privilege escalation: you cannot grant a role whose permissions
//	@Description	exceed your own (unless you are SUPER_ADMIN).
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"User ID"
//	@Param			payload	body		userdto.AssignRoleRequest	true	"Assign role payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		403		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/roles [post]
func (h *Handler) AssignRole(c *gin.Context) {
	var req dto.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := c.Request.Context()
	targetID := c.Param("id")

	// Privilege-escalation guard: resolve the role's permissions and make sure
	// the actor holds all of them.
	role, err := h.rbac.GetRole(ctx, req.RoleID)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "unknown role")
		return
	}
	codes := make([]string, 0, len(role.Permissions))
	for _, p := range role.Permissions {
		codes = append(codes, p.Code)
	}
	if err := h.rbac.EnsureCanGrant(ctx, c.GetString("user_id"), rolesFromCtx(c), codes); err != nil {
		h.recDenied(c, types.AuditUserRoleDenied, "user_role", targetID,
			"Denied grant of role "+role.Code+" (privilege escalation)")
		httpx.Error(c, http.StatusForbidden, "cannot grant a role with permissions you do not hold")
		return
	}

	// Record who performed the grant.
	if req.AssignedBy == nil {
		if actor := c.GetString("user_id"); actor != "" {
			req.AssignedBy = &actor
		}
	}

	if err := h.service.AssignRole(ctx, targetID, req); err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.rec(c, types.AuditUserRoleGrant, "user_role", targetID,
		"Granted role "+role.Code+" (scope "+req.ScopeType+")", nil,
		map[string]any{"role_id": req.RoleID, "role_code": role.Code, "scope_type": req.ScopeType, "scope_id": req.ScopeID})
	httpx.OK(c, gin.H{"message": "role assigned"})
}

// RemoveRole godoc
//
//	@Summary		Remove role from user
//	@Description	Deactivates a role assignment for a user
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string	true	"User ID"
//	@Param			roleId	path		string	true	"Role ID"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/roles/{roleId} [delete]
func (h *Handler) RemoveRole(c *gin.Context) {
	targetID, roleID := c.Param("id"), c.Param("roleId")
	if err := h.service.RemoveRole(c.Request.Context(), targetID, roleID); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.rec(c, types.AuditUserRoleRevoke, "user_role", targetID, "Revoked role from user", nil,
		map[string]any{"role_id": roleID})
	httpx.OK(c, gin.H{"message": "role removed"})
}

// AssignUser godoc
//
//	@Summary		Assign user to organization scope
//	@Description	Assigns user to district, subcounty, or facility
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"User ID"
//	@Param			payload	body		userdto.AssignUserRequest	true	"Assignment payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/assignments [post]
func (h *Handler) AssignUser(c *gin.Context) {
	var req dto.AssignUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.AssignUser(c.Request.Context(), c.Param("id"), req); err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.rec(c, types.AuditUserAssignScope, "user", c.Param("id"),
		"Assigned user to "+req.AssignmentLevel+" scope", nil, req)
	httpx.OK(c, gin.H{"message": "assignment created"})
}

// UpdateAssignment godoc
//
//	@Summary		Update user assignment
//	@Description	Updates a user assignment record
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			assignmentId	path		string						true	"Assignment ID"
//	@Param			payload			body		userdto.AssignUserRequest	true	"Assignment payload"
//	@Success		200				{object}	map[string]interface{}
//	@Failure		400				{object}	map[string]interface{}
//	@Failure		500				{object}	map[string]interface{}
//	@Router			/users/assignments/{assignmentId} [patch]
func (h *Handler) UpdateAssignment(c *gin.Context) {
	var req dto.AssignUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.UpdateAssignment(c.Request.Context(), c.Param("assignmentId"), req); err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.rec(c, types.AuditUserAssignScope, "user_assignment", c.Param("assignmentId"),
		"Updated user assignment", nil, req)
	httpx.OK(c, gin.H{"message": "assignment updated"})
}

// AssignCapability godoc
//
//	@Summary		Assign capability to user
//	@Description	Assigns a capability to a user
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"User ID"
//	@Param			payload	body		userdto.AssignCapabilityRequest	true	"Capability payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/capabilities [post]
func (h *Handler) AssignCapability(c *gin.Context) {
	var req dto.AssignCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.AssignCapability(c.Request.Context(), c.Param("id"), req); err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.rec(c, types.AuditUserUpdate, "user", c.Param("id"), "Assigned capability to user", nil, req)
	httpx.OK(c, gin.H{"message": "capability assigned"})
}

// UpdateCapability godoc
//
//	@Summary		Update user capability
//	@Description	Updates a user capability record
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			capabilityRecordId	path		string							true	"User capability record ID"
//	@Param			payload				body		userdto.AssignCapabilityRequest	true	"Capability payload"
//	@Success		200					{object}	map[string]interface{}
//	@Failure		400					{object}	map[string]interface{}
//	@Failure		500					{object}	map[string]interface{}
//	@Router			/users/capabilities/{capabilityRecordId} [patch]
func (h *Handler) UpdateCapability(c *gin.Context) {
	var req dto.AssignCapabilityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.UpdateCapability(c.Request.Context(), c.Param("capabilityRecordId"), req); err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.rec(c, types.AuditUserUpdate, "user_capability", c.Param("capabilityRecordId"),
		"Updated user capability", nil, req)
	httpx.OK(c, gin.H{"message": "capability updated"})
}

// UpdateProfile godoc
//
//	@Summary		Update user profile
//	@Description	Updates profile details for a user
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string								true	"User ID"
//	@Param			payload	body		userdto.UpdateUserProfileRequest	true	"Update profile payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/profile [patch]
func (h *Handler) UpdateProfile(c *gin.Context) {
	var req dto.UpdateUserProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.UpdateProfile(c.Request.Context(), c.Param("id"), req); err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	h.rec(c, types.AuditUserProfileUpdate, "user", c.Param("id"), "Updated user profile", nil, req)
	httpx.OK(c, gin.H{"message": "profile updated"})
}

// GetDetails godoc
//
//	@Summary		Get user details
//	@Description	Returns user, profile, roles, assignments, and capabilities
//	@Tags			Users
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/users/{id}/details [get]
func (h *Handler) GetDetails(c *gin.Context) {
	out, err := h.service.GetDetails(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// --- helpers ---

func rolesFromCtx(c *gin.Context) []string {
	if v, ok := c.Get("roles"); ok {
		if r, ok := v.([]string); ok {
			return r
		}
	}
	return nil
}

// hasPermission checks a global permission for the current actor via the RBAC
// service. Returns false on any error (fail-closed).
func (h *Handler) hasPermission(c *gin.Context, permission string) bool {
	if h.rbac == nil {
		return false
	}
	ok, err := h.rbac.HasPermission(c.Request.Context(), c.GetString("user_id"), permission, "GLOBAL", nil)
	return err == nil && ok
}

func (h *Handler) rec(c *gin.Context, action, entityType, entityID, desc string, before, after any) {
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   c.GetString("user_id"),
		ActorUsername: c.GetString("username"),
		ActorRoles:    rolesFromCtx(c),
		Action:        action,
		EntityType:    entityType,
		EntityID:      entityID,
		Status:        types.AuditStatusSuccess,
		Description:   desc,
		Before:        before,
		After:         after,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
}

func (h *Handler) recDenied(c *gin.Context, action, entityType, entityID, desc string) {
	c.Set("audit_denial_recorded", true)
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   c.GetString("user_id"),
		ActorUsername: c.GetString("username"),
		ActorRoles:    rolesFromCtx(c),
		Action:        action,
		EntityType:    entityType,
		EntityID:      entityID,
		Status:        types.AuditStatusDenied,
		Description:   desc,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})
}
