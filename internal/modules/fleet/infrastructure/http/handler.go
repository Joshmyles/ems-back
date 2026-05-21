package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	fleetapp "dispatch/internal/modules/fleet/application"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

// driverScopeUserID returns the user ID to scope ambulance reads to when the
// caller's only role is DRIVER. A driver should only see the ambulance they
// are the active driver on. Any non-DRIVER role grants the broader view and
// this returns nil.
func driverScopeUserID(c *gin.Context) *string {
	rawRoles, _ := c.Get("roles")
	roles, _ := rawRoles.([]string)
	hasDriver := false
	for _, r := range roles {
		code := strings.ToUpper(strings.TrimSpace(r))
		if code == "DRIVER" {
			hasDriver = true
			continue
		}
		return nil
	}
	if !hasDriver {
		return nil
	}
	if uid := c.GetString("user_id"); uid != "" {
		return &uid
	}
	return nil
}

type Handler struct {
	service *fleetapp.Service
}

func NewHandler(service *fleetapp.Service) *Handler {
	return &Handler{service: service}
}

// List godoc
//
//	@Summary		List ambulances
//	@Description	Returns paginated ambulances with status and readiness
//	@Tags			Fleet
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page						query		int		false	"Page number"	default(1)
//	@Param			page_size					query		int		false	"Page size"		default(20)
//	@Param			search						query		string	false	"Search term (code, plate, VIN, make, model)"
//	@Param			sort_by						query		string	false	"Sort field"					Enums(created_at,plate_number,status,dispatch_readiness)
//	@Param			sort_order					query		string	false	"Sort order"					Enums(ASC,DESC)
//	@Param			filter[status]				query		string	false	"Filter by status"				Enums(AVAILABLE,RESERVED,ASSIGNED,ENROUTE,AT_SCENE,TRANSPORTING,RETURNING,MAINTENANCE,BREAKDOWN,OFFLINE,RETIRED)
//	@Param			filter[dispatch_readiness]	query		string	false	"Filter by dispatch readiness"	Enums(DISPATCHABLE,RESTRICTED,NOT_DISPATCHABLE)
//	@Param			filter[district_id]			query		string	false	"Filter by district id"
//	@Param			filter[category_id]			query		string	false	"Filter by ambulance category id"
//	@Param			filter[date_from]			query		string	false	"Filter by created_at from (ISO 8601)"
//	@Param			filter[date_to]				query		string	false	"Filter by created_at to (ISO 8601)"
//	@Success		200							{object}	map[string]interface{}
//	@Failure		500							{object}	map[string]interface{}
//	@Router			/ambulances [get]
func (h *Handler) List(c *gin.Context) {
	p := platformdb.ParsePagination(
		c.Request.URL.Query(),
		map[string]string{
			"created_at":         "a.created_at",
			"plate_number":       "a.plate_number",
			"status":             "a.status",
			"dispatch_readiness": "a.dispatch_readiness",
		},
		map[string]struct{}{
			"status":             {},
			"dispatch_readiness": {},
			"district_id":        {},
			"category_id":        {},
			"date_from":          {},
			"date_to":            {},
		},
	)
	out, err := h.service.ListAmbulances(c.Request.Context(), p, driverScopeUserID(c))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Get godoc
//
//	@Summary		Get ambulance
//	@Description	Get a single ambulance by ID
//	@Tags			Fleet
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Ambulance ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/ambulances/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	out, err := h.service.GetAmbulance(c.Request.Context(), id, driverScopeUserID(c))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Create godoc
//
//	@Summary		Create ambulance
//	@Description	Create a new ambulance record
//	@Tags			Fleet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		fleetapp.CreateAmbulanceRequest	true	"Create ambulance payload"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/ambulances [post]
func (h *Handler) Create(c *gin.Context) {
	var req fleetapp.CreateAmbulanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.CreateAmbulance(c.Request.Context(), req)
	if err != nil {
		httpx.DBError(c, err)
		return
	}
	httpx.Created(c, out)
}

// Update godoc
//
//	@Summary		Update ambulance
//	@Description	Update an existing ambulance
//	@Tags			Fleet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"Ambulance ID"
//	@Param			payload	body		fleetapp.UpdateAmbulanceRequest	true	"Update ambulance payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/ambulances/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req fleetapp.UpdateAmbulanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.UpdateAmbulance(c.Request.Context(), id, req)
	if err != nil {
		httpx.DBError(c, err)
		return
	}
	httpx.OK(c, out)
}

// AssignDriver godoc
//
//	@Summary		Assign driver to ambulance
//	@Description	Sets the active driver on an ambulance's crew assignment
//	@Tags			Fleet
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string						true	"Ambulance ID"
//	@Param			payload	body		fleetapp.AssignDriverRequest	true	"Driver assignment payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		404		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/ambulances/{id}/driver [post]
func (h *Handler) AssignDriver(c *gin.Context) {
	id := c.Param("id")
	var req fleetapp.AssignDriverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.AssignDriverToAmbulance(c.Request.Context(), id, req)
	if err != nil {
		httpx.DBError(c, err)
		return
	}
	httpx.OK(c, out)
}

// UnassignDriver godoc
//
//	@Summary		Unassign driver from ambulance
//	@Description	Clears the driver from the ambulance's active crew assignment
//	@Tags			Fleet
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Ambulance ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/ambulances/{id}/driver [delete]
func (h *Handler) UnassignDriver(c *gin.Context) {
	id := c.Param("id")
	out, err := h.service.UnassignDriverFromAmbulance(c.Request.Context(), id)
	if err != nil {
		httpx.DBError(c, err)
		return
	}
	httpx.OK(c, out)
}

// Delete godoc
//
//	@Summary		Delete ambulance
//	@Description	Delete an ambulance by ID
//	@Tags			Fleet
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Ambulance ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/ambulances/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteAmbulance(c.Request.Context(), id); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, gin.H{"message": "ambulance deleted"})
}
