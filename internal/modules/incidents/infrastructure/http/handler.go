package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	incidentapp "dispatch/internal/modules/incidents/application"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct{ service *incidentapp.Service }

func NewHandler(service *incidentapp.Service) *Handler { return &Handler{service: service} }

// Create godoc
//
//	@Summary		Create incident with triage
//	@Description	Creates an incident and optionally persists triage responses on creation
//	@Tags			Incidents
//	@Accept			json
//	@Produce		json
//	@Param			payload	body		incidentapp.CreateIncidentRequest	true	"Incident payload"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/incidents [post]
func (h *Handler) Create(c *gin.Context) {
	var req incidentapp.CreateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.CreatedByUserID == nil {
		if v := c.GetString("user_id"); v != "" {
			req.CreatedByUserID = &v
		}
	}
	out, err := h.service.CreateIncident(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
}

// List godoc
//
//	@Summary		List incidents
//	@Description	Returns paginated incidents
//	@Tags			Incidents
//	@Produce		json
//	@Security		BearerAuth
//	@Param			status		query		string	false	"Incident status"
//	@Param			district_id	query		string	false	"District ID"
//	@Param			facility_id	query		string	false	"Facility ID"
//	@Param			priority_id	query		string	false	"Priority level ID"
//	@Param			page		query		int		false	"Page number"	default(1)
//	@Param			page_size	query		int		false	"Page size"		default(20)
//	@Param			search		query		string	false	"Search by incident number, summary, or patient name"
//	@Param			sort_by		query		string	false	"Sort field"	Enums(reported_at,created_at,status)
//	@Param			sort_order	query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Success		200			{object}	map[string]interface{}
//	@Failure		500			{object}	map[string]interface{}
//	@Router			/incidents [get]
func (h *Handler) List(c *gin.Context) {
	var status, districtID, facilityID, priorityID *string
	if v := c.Query("status"); v != "" {
		status = &v
	}
	if v := c.Query("district_id"); v != "" {
		districtID = &v
	}
	if v := c.Query("facility_id"); v != "" {
		facilityID = &v
	}
	if v := c.Query("priority_id"); v != "" {
		priorityID = &v
	}
	params := incidentapp.ListIncidentsParams{Status: status, DistrictID: districtID, FacilityID: facilityID, PriorityID: priorityID,
		Pagination: platformdb.ParsePagination(c.Request.URL.Query(), map[string]string{"reported_at": "i.reported_at", "created_at": "i.created_at", "status": "i.status"}, map[string]struct{}{}),
	}
	out, err := h.service.ListIncidents(c.Request.Context(), params)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Update godoc
//
//	@Summary		Update incident
//	@Description	Updates incident attributes
//	@Tags			Incidents
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string							true	"Incident ID"
//	@Param			payload	body		incidentapp.UpdateIncidentRequest	true	"Incident update payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/incidents/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	var req incidentapp.UpdateIncidentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var actorUserID *string
	if v := c.GetString("user_id"); v != "" {
		actorUserID = &v
	}
	out, err := h.service.UpdateIncident(c.Request.Context(), c.Param("id"), req, actorUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// UpdateStatus godoc
//
//	@Summary		Update incident status
//	@Description	Updates incident lifecycle status
//	@Tags			Incidents
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string									true	"Incident ID"
//	@Param			payload	body		incidentapp.UpdateIncidentStatusRequest	true	"Incident status payload"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/incidents/{id}/status [patch]
func (h *Handler) UpdateStatus(c *gin.Context) {
	var req incidentapp.UpdateIncidentStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	var actorUserID *string
	if v := c.GetString("user_id"); v != "" {
		actorUserID = &v
	}
	out, err := h.service.UpdateIncidentStatus(c.Request.Context(), c.Param("id"), req, actorUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// GetByID godoc
//
//	@Summary		Get incident by ID
//	@Description	Returns an incident by ID
//	@Tags			Incidents
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Incident ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/incidents/{id} [get]
func (h *Handler) GetByID(c *gin.Context) {
	out, err := h.service.GetIncidentByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}
