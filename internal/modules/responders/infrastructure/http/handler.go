package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	respapp "dispatch/internal/modules/responders/application"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *respapp.Service
}

func NewHandler(service *respapp.Service) *Handler {
	return &Handler{service: service}
}

// List godoc
//
//	@Summary		List responders
//	@Description	Returns paginated responders aggregated from ambulances, their active crew/driver, station and current dispatch load
//	@Tags			Responders
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page				query		int		false	"Page number"	default(1)
//	@Param			page_size			query		int		false	"Page size"		default(20)
//	@Param			search				query		string	false	"Search term (code, plate, driver name, district)"
//	@Param			sort_by				query		string	false	"Sort field"	Enums(created_at,plate_number,status)
//	@Param			sort_order			query		string	false	"Sort order"	Enums(ASC,DESC)
//	@Param			filter[status]		query		string	false	"Filter by ambulance status"
//	@Param			filter[district_id]	query		string	false	"Filter by district id"
//	@Param			filter[category_id]	query		string	false	"Filter by ambulance category id"
//	@Success		200					{object}	map[string]interface{}
//	@Failure		500					{object}	map[string]interface{}
//	@Router			/responders [get]
func (h *Handler) List(c *gin.Context) {
	p := platformdb.ParsePagination(
		c.Request.URL.Query(),
		map[string]string{
			"created_at":   "a.created_at",
			"plate_number": "a.plate_number",
			"status":       "a.status",
		},
		map[string]struct{}{
			"status":      {},
			"district_id": {},
			"category_id": {},
		},
	)
	out, err := h.service.ListResponders(c.Request.Context(), p)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Get godoc
//
//	@Summary		Get responder
//	@Description	Get a single responder by ambulance ID
//	@Tags			Responders
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Ambulance ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Router			/responders/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	out, err := h.service.GetResponder(c.Request.Context(), c.Param("id"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, err.Error())
		return
	}
	httpx.OK(c, out)
}
