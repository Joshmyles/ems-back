package http

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	incidentapp "dispatch/internal/modules/incidents/application"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

// responderRoles are roles whose members may only see incidents assigned to
// them. seeAllRoles take precedence: a user holding any seeAll role sees every
// incident regardless of also being a responder.
var responderRoles = map[string]struct{}{
	"DRIVER": {},
	"MEDIC":  {},
}

// assignedScopeUserID returns the user ID to scope the incident list to when
// the caller is a pure responder (driver/medic) with no broader role. It
// returns nil when the caller should see all incidents.
func assignedScopeUserID(c *gin.Context) *string {
	rawRoles, _ := c.Get("roles")
	roles, _ := rawRoles.([]string)
	hasResponderRole := false
	for _, r := range roles {
		code := strings.ToUpper(strings.TrimSpace(r))
		if _, ok := responderRoles[code]; ok {
			hasResponderRole = true
			continue
		}
		// Any non-responder role grants the broader incidents view.
		return nil
	}
	if !hasResponderRole {
		return nil
	}
	if uid := c.GetString("user_id"); uid != "" {
		return &uid
	}
	return nil
}

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
//	@Param			status					query		string	false	"Incident status"
//	@Param			district_id				query		string	false	"District ID"
//	@Param			receiving_facility_id	query		string	false	"Receiving (destination) facility ID"
//	@Param			referring_facility_id	query		string	false	"Referring (origin) facility ID"
//	@Param			priority_id				query		string	false	"Priority level ID"
//	@Param			date_from				query		string	false	"Reported-at start date/time (YYYY-MM-DD or RFC3339)"
//	@Param			date_to					query		string	false	"Reported-at end date/time, exclusive (YYYY-MM-DD or RFC3339)"
//	@Param			page					query		int		false	"Page number"	default(1)
//	@Param			page_size				query		int		false	"Page size"		default(20)
//	@Param			search					query		string	false	"Search by incident number, summary, or patient name"
//	@Param			sort_by					query		string	false	"Sort field"	Enums(reported_at,created_at,status)
//	@Param			sort_order				query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Success		200						{object}	map[string]interface{}
//	@Failure		500						{object}	map[string]interface{}
//	@Router			/incidents [get]
func (h *Handler) List(c *gin.Context) {
	var status, districtID, receivingFacilityID, referringFacilityID, priorityID *string
	if v := c.Query("status"); v != "" {
		status = &v
	}
	if v := c.Query("district_id"); v != "" {
		districtID = &v
	}
	if v := c.Query("receiving_facility_id"); v != "" {
		receivingFacilityID = &v
	}
	if v := c.Query("referring_facility_id"); v != "" {
		referringFacilityID = &v
	}
	if v := c.Query("priority_id"); v != "" {
		priorityID = &v
	}
	dateFrom, err := parseIncidentDateQuery(c.Query("date_from"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid date_from")
		return
	}
	dateTo, err := parseIncidentDateQuery(c.Query("date_to"))
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid date_to")
		return
	}
	params := incidentapp.ListIncidentsParams{Status: status, DistrictID: districtID,
		ReceivingFacilityID: receivingFacilityID, ReferringFacilityID: referringFacilityID, PriorityID: priorityID,
		DateFrom:         dateFrom,
		DateTo:           dateTo,
		AssignedToUserID: assignedScopeUserID(c),
		Pagination:       platformdb.ParsePagination(c.Request.URL.Query(), map[string]string{"reported_at": "i.reported_at", "created_at": "i.created_at", "status": "i.status"}, map[string]struct{}{}),
	}
	out, err := h.service.ListIncidents(c.Request.Context(), params)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

func parseIncidentDateQuery(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return &t, nil
	}
	t, err := time.ParseInLocation("2006-01-02", raw, time.Local)
	if err != nil {
		return nil, err
	}
	return &t, nil
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
	if scopeUserID := assignedScopeUserID(c); scopeUserID != nil {
		out, err := h.service.UpdateIncidentStatusForAssignee(c.Request.Context(), c.Param("id"), *scopeUserID, req, actorUserID)
		if err != nil {
			if errors.Is(err, incidentapp.ErrIncidentNotAssigned) {
				httpx.Error(c, http.StatusForbidden, "incident not assigned to you")
				return
			}
			httpx.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.OK(c, out)
		return
	}
	out, err := h.service.UpdateIncidentStatus(c.Request.Context(), c.Param("id"), req, actorUserID)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Delete godoc
//
//	@Summary		Delete incident
//	@Description	Hard-deletes an incident. Restricted to administrators (incidents.delete permission).
//	@Tags			Incidents
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Incident ID"
//	@Success		204	"No Content"
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/incidents/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	var actorUserID *string
	if v := c.GetString("user_id"); v != "" {
		actorUserID = &v
	}
	if err := h.service.DeleteIncident(c.Request.Context(), c.Param("id"), actorUserID); err != nil {
		if errors.Is(err, incidentapp.ErrIncidentNotFound) {
			httpx.Error(c, http.StatusNotFound, "incident not found")
			return
		}
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.Status(http.StatusNoContent)
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
	id := c.Param("id")
	// Pure responders (driver/medic) may only view incidents assigned to them.
	if scopeUserID := assignedScopeUserID(c); scopeUserID != nil {
		out, err := h.service.GetIncidentByIDForAssignee(c.Request.Context(), id, *scopeUserID)
		if err != nil {
			if errors.Is(err, incidentapp.ErrIncidentNotAssigned) {
				httpx.Error(c, http.StatusForbidden, "incident not assigned to you")
				return
			}
			httpx.Error(c, http.StatusInternalServerError, err.Error())
			return
		}
		httpx.OK(c, out)
		return
	}
	out, err := h.service.GetIncidentByID(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}
