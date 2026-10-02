package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	tripsapp "dispatch/internal/modules/trips/application"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *tripsapp.Service
}

func NewHandler(service *tripsapp.Service) *Handler {
	return &Handler{service: service}
}

// List godoc
//
//	@Summary	List trips
//	@Tags		Trips
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page					query		int		false	"Page number"	default(1)
//	@Param		page_size				query		int		false	"Page size"		default(20)
//	@Param		sort_by					query		string	false	"Sort field"	Enums(started_at,created_at)
//	@Param		sort_order				query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Param		filter[incident_id]		query		string	false	"Filter by incident id"
//	@Param		filter[ambulance_id]	query		string	false	"Filter by ambulance id"
//	@Param		filter[date_from]		query		string	false	"Filter by started_at from (ISO 8601)"
//	@Param		filter[date_to]			query		string	false	"Filter by started_at to (ISO 8601)"
//	@Param		filter[state]			query		string	false	"Derived trip state"	Enums(active,completed,cancelled)
//	@Param		search					query		string	false	"Search incident, ambulance, destination, crew, outcome or notes"
//	@Success	200						{object}	map[string]interface{}
//	@Failure	500						{object}	map[string]interface{}
//	@Router		/trips [get]
func (h *Handler) List(c *gin.Context) {
	p := platformdb.ParsePagination(
		c.Request.URL.Query(),
		map[string]string{
			"started_at": "t.started_at",
			"created_at": "t.created_at",
		},
		map[string]struct{}{
			"incident_id":  {},
			"ambulance_id": {},
			"date_from":    {},
			"date_to":      {},
			"state":        {},
		},
	)
	out, err := h.service.List(c.Request.Context(), p)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Summary godoc
//
//	@Summary	Summarize trips
//	@Tags		Trips
//	@Produce	json
//	@Security	BearerAuth
//	@Success	200	{object}	map[string]interface{}
//	@Failure	500	{object}	map[string]interface{}
//	@Router		/trips/summary [get]
func (h *Handler) Summary(c *gin.Context) {
	out, err := h.service.Summary(c.Request.Context())
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Get godoc
//
//	@Summary	Get trip
//	@Tags		Trips
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Trip ID"
//	@Success	200	{object}	map[string]interface{}
//	@Failure	404	{object}	map[string]interface{}
//	@Router		/trips/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	out, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		httpx.Error(c, http.StatusNotFound, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Create godoc
//
//	@Summary	Create trip
//	@Tags		Trips
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		payload	body		tripsapp.CreateTripRequest	true	"Create trip payload"
//	@Success	201		{object}	map[string]interface{}
//	@Failure	400		{object}	map[string]interface{}
//	@Failure	500		{object}	map[string]interface{}
//	@Router		/trips [post]
func (h *Handler) Create(c *gin.Context) {
	var req tripsapp.CreateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.Create(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
}

// Update godoc
//
//	@Summary	Update trip
//	@Tags		Trips
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string						true	"Trip ID"
//	@Param		payload	body		tripsapp.UpdateTripRequest	true	"Update trip payload"
//	@Success	200		{object}	map[string]interface{}
//	@Failure	400		{object}	map[string]interface{}
//	@Failure	500		{object}	map[string]interface{}
//	@Router		/trips/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req tripsapp.UpdateTripRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.Update(c.Request.Context(), id, req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Delete godoc
//
//	@Summary	Delete trip
//	@Tags		Trips
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Trip ID"
//	@Success	200	{object}	map[string]interface{}
//	@Failure	500	{object}	map[string]interface{}
//	@Router		/trips/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(c.Request.Context(), id); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, gin.H{"message": "trip deleted"})
}

// ListEvents godoc
//
//	@Summary	List trip events
//	@Tags		Trips
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id			path		string	true	"Trip ID"
//	@Param		page		query		int		false	"Page number"	default(1)
//	@Param		page_size	query		int		false	"Page size"		default(20)
//	@Success	200			{object}	map[string]interface{}
//	@Failure	500			{object}	map[string]interface{}
//	@Router		/trips/{id}/events [get]
func (h *Handler) ListEvents(c *gin.Context) {
	tripID := c.Param("id")
	p := platformdb.ParsePagination(
		c.Request.URL.Query(),
		map[string]string{
			"created_at": "te.event_time",
		},
		map[string]struct{}{},
	)
	out, err := h.service.ListEvents(c.Request.Context(), tripID, p)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// CreateEvent godoc
//
//	@Summary	Create trip event
//	@Tags		Trips
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id		path		string							true	"Trip ID"
//	@Param		payload	body		tripsapp.CreateTripEventRequest	true	"Create trip event payload"
//	@Success	201		{object}	map[string]interface{}
//	@Failure	400		{object}	map[string]interface{}
//	@Failure	500		{object}	map[string]interface{}
//	@Router		/trips/{id}/events [post]
func (h *Handler) CreateEvent(c *gin.Context) {
	tripID := c.Param("id")
	var req tripsapp.CreateTripEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.CreateEvent(c.Request.Context(), tripID, req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
}
