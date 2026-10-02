package http

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
	rbacinfra "dispatch/internal/modules/rbac/infrastructure"
	"dispatch/internal/platform/httpx"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/shared/types"
)

type Handler struct {
	service *rbacapp.Service
	audit   types.AuditRecorder
}

func NewHandler(service *rbacapp.Service, audit types.AuditRecorder) *Handler {
	if audit == nil {
		audit = types.NoopAuditRecorder{}
	}
	return &Handler{service: service, audit: audit}
}

// MyPermissions godoc
//
//	@Summary		Get my permissions
//	@Description	Returns the list of permissions granted to the authenticated user
//	@Tags			RBAC
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/rbac/me/permissions [get]
func (h *Handler) MyPermissions(c *gin.Context) {
	userID := c.GetString("user_id")
	items, err := h.service.ListPermissionGrants(c.Request.Context(), userID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, items)
}

// ListPermissions godoc
//
//	@Summary		List the permission catalogue
//	@Tags			RBAC
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]interface{}
//	@Router			/rbac/permissions [get]
func (h *Handler) ListPermissions(c *gin.Context) {
	items, err := h.service.ListPermissions(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, items)
}

// ListRoles godoc
//
//	@Summary		List roles
//	@Tags			RBAC
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]interface{}
//	@Router			/rbac/roles [get]
func (h *Handler) ListRoles(c *gin.Context) {
	p := platformdb.ParsePagination(c.Request.URL.Query(),
		map[string]string{"name": "r.name", "code": "r.code", "created_at": "r.created_at"},
		map[string]struct{}{})
	out, err := h.service.ListRoles(c.Request.Context(), p)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// GetRole godoc
//
//	@Summary		Get a role with its permissions
//	@Tags			RBAC
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Role ID"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/rbac/roles/{id} [get]
func (h *Handler) GetRole(c *gin.Context) {
	role, err := h.service.GetRole(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, rbacinfra.ErrRoleNotFound) {
			httpx.Error(c, http.StatusNotFound, "role not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, role)
}

type createRoleRequest struct {
	Code          string   `json:"code" binding:"required"`
	Name          string   `json:"name" binding:"required"`
	Description   string   `json:"description"`
	PermissionIDs []string `json:"permission_ids"`
}

// CreateRole godoc
//
//	@Summary		Create a custom role
//	@Tags			RBAC
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body	createRoleRequest	true	"Role payload"
//	@Success		201	{object}	map[string]interface{}
//	@Router			/rbac/roles [post]
func (h *Handler) CreateRole(c *gin.Context) {
	var req createRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := c.Request.Context()
	actorID, actorRoles := c.GetString("user_id"), rolesFromCtx(c)

	// Guard: the initial permission set may not exceed what the actor holds.
	if len(req.PermissionIDs) > 0 {
		codes, err := h.permissionCodesForIDs(ctx, req.PermissionIDs)
		if err != nil {
			httpx.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		if err := h.service.EnsureCanGrant(ctx, actorID, actorRoles, codes); err != nil {
			h.denyEscalation(c, "", req.Name, err)
			return
		}
	}

	role, err := h.service.CreateRole(ctx, req.Code, req.Name, req.Description)
	if err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	if len(req.PermissionIDs) > 0 {
		if err := h.service.SetRolePermissions(ctx, role.ID, req.PermissionIDs); err != nil {
			httpx.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		role.PermissionCount = len(req.PermissionIDs)
	}

	h.audit.Record(ctx, h.entry(c, types.AuditRoleCreate, "role", role.ID,
		"Created role "+role.Code, nil, role))
	httpx.Created(c, role)
}

type updateRoleRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateRole godoc
//
//	@Summary		Update a role's name/description
//	@Tags			RBAC
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	string				true	"Role ID"
//	@Param			payload	body	updateRoleRequest	true	"Role payload"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/rbac/roles/{id} [put]
func (h *Handler) UpdateRole(c *gin.Context) {
	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := c.Request.Context()
	id := c.Param("id")

	before, err := h.service.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, rbacinfra.ErrRoleNotFound) {
			httpx.Error(c, http.StatusNotFound, "role not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.service.UpdateRole(ctx, id, req.Name, req.Description); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	after, _ := h.service.GetRole(ctx, id)
	h.audit.Record(ctx, h.entry(c, types.AuditRoleUpdate, "role", id,
		"Updated role "+before.Code, before.Role, after.Role))
	httpx.OK(c, after)
}

// DeleteRole godoc
//
//	@Summary		Delete a custom role
//	@Tags			RBAC
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Role ID"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/rbac/roles/{id} [delete]
func (h *Handler) DeleteRole(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")

	isSystem, err := h.service.IsSystemRole(ctx, id)
	if err != nil {
		if errors.Is(err, rbacinfra.ErrRoleNotFound) {
			httpx.Error(c, http.StatusNotFound, "role not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if isSystem {
		httpx.Error(c, http.StatusForbidden, "system roles cannot be deleted")
		return
	}

	before, _ := h.service.GetRole(ctx, id)
	if err := h.service.DeleteRole(ctx, id); err != nil {
		switch {
		case errors.Is(err, rbacinfra.ErrRoleInUse):
			httpx.Error(c, http.StatusConflict, "role is still assigned to users")
		case errors.Is(err, rbacinfra.ErrRoleNotFound):
			httpx.Error(c, http.StatusNotFound, "role not found")
		default:
			httpx.Error(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	h.audit.Record(ctx, h.entry(c, types.AuditRoleDelete, "role", id,
		"Deleted role "+before.Code, before.Role, nil))
	httpx.OK(c, gin.H{"message": "role deleted"})
}

type setPermissionsRequest struct {
	PermissionIDs []string `json:"permission_ids"`
}

// SetRolePermissions godoc
//
//	@Summary		Replace a role's permission set
//	@Tags			RBAC
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path	string					true	"Role ID"
//	@Param			payload	body	setPermissionsRequest	true	"Permission ids"
//	@Success		200	{object}	map[string]interface{}
//	@Router			/rbac/roles/{id}/permissions [put]
func (h *Handler) SetRolePermissions(c *gin.Context) {
	var req setPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	ctx := c.Request.Context()
	id := c.Param("id")
	actorID, actorRoles := c.GetString("user_id"), rolesFromCtx(c)

	before, err := h.service.GetRole(ctx, id)
	if err != nil {
		if errors.Is(err, rbacinfra.ErrRoleNotFound) {
			httpx.Error(c, http.StatusNotFound, "role not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Guard against privilege escalation: you can't attach a permission you
	// don't yourself hold.
	codes, err := h.permissionCodesForIDs(ctx, req.PermissionIDs)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.service.EnsureCanGrant(ctx, actorID, actorRoles, codes); err != nil {
		h.denyEscalation(c, id, before.Code, err)
		return
	}

	if err := h.service.SetRolePermissions(ctx, id, req.PermissionIDs); err != nil {
		if httpx.DBError(c, err) {
			return
		}
		return
	}
	after, _ := h.service.GetRole(ctx, id)
	h.audit.Record(ctx, h.entry(c, types.AuditRolePermsSet, "role", id,
		"Set permissions on role "+before.Code, before.Permissions, after.Permissions))
	httpx.OK(c, after)
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

// permissionCodesForIDs maps permission ids to their codes via the catalogue.
func (h *Handler) permissionCodesForIDs(ctx context.Context, ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	all, err := h.service.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]string, len(all))
	for _, p := range all {
		byID[p.ID] = p.Code
	}
	codes := make([]string, 0, len(ids))
	for _, id := range ids {
		if code, ok := byID[id]; ok {
			codes = append(codes, code)
		}
	}
	return codes, nil
}

func (h *Handler) denyEscalation(c *gin.Context, roleID, roleName string, err error) {
	c.Set("audit_denial_recorded", true)
	var esc *rbacapp.EscalationError
	msg := "cannot grant permissions you do not hold"
	meta := map[string]any{}
	if errors.As(err, &esc) {
		meta["missing_permissions"] = esc.Missing
	}
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   c.GetString("user_id"),
		ActorUsername: c.GetString("username"),
		ActorRoles:    rolesFromCtx(c),
		Action:        types.AuditRolePermsSet,
		EntityType:    "role",
		EntityID:      roleID,
		Status:        types.AuditStatusDenied,
		Description:   "Denied permission change on role " + roleName + " (privilege escalation)",
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		Metadata:      meta,
	})
	httpx.Error(c, http.StatusForbidden, msg)
}

// entry builds a success-status audit entry from the request context.
func (h *Handler) entry(c *gin.Context, action, entityType, entityID, desc string, before, after any) types.AuditEntry {
	return types.AuditEntry{
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
	}
}
