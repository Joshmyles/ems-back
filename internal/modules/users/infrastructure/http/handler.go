package http

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	userSv "dispatch/internal/modules/users/application"
	"dispatch/internal/modules/users/application/dto"
	userdto "dispatch/internal/modules/users/application/dto"
	"dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *userSv.Service
}

func NewHandler(service *userSv.Service) *Handler {
	return &Handler{service: service}
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
	user, err := h.service.Update(c.Request.Context(), c.Param("id"), req)
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
	err := h.service.Delete(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, userSv.ErrUserNotFound) {
			httpx.Error(c, http.StatusNotFound, err.Error())
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, gin.H{"message": "user deleted"})
}

// ChangePassword godoc
//
//	@Summary		Change user password
//	@Description	Changes a user's password using current password or admin reset flow
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"User ID"
//	@Param			payload	body		userdto.ChangePasswordRequest	true	"Change password payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/change-password [post]
func (h *Handler) ChangePassword(c *gin.Context) {
	var req dto.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.ChangePassword(c.Request.Context(), c.Param("id"), req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	httpx.OK(c, gin.H{"message": "password changed successfully"})
}

// AssignRole godoc
//
//	@Summary		Assign role to user
//	@Description	Assigns a role to a user with optional scope
//	@Tags			Users
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"User ID"
//	@Param			payload	body		userdto.AssignRoleRequest	true	"Assign role payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/users/{id}/roles [post]
func (h *Handler) AssignRole(c *gin.Context) {
	var req dto.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.AssignRole(c.Request.Context(), c.Param("id"), req); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
	if err := h.service.RemoveRole(c.Request.Context(), c.Param("id"), c.Param("roleId")); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
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
