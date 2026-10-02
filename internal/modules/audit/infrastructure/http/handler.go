package http

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	auditapp "dispatch/internal/modules/audit/application"
	auditdomain "dispatch/internal/modules/audit/domain"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *auditapp.Service
}

func NewHandler(service *auditapp.Service) *Handler { return &Handler{service: service} }

// List godoc
//
//	@Summary		List audit logs
//	@Description	Returns a filterable, paginated view of the security audit trail
//	@Tags			Audit
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page			query	int		false	"Page number"
//	@Param			page_size		query	int		false	"Page size"
//	@Param			search			query	string	false	"Free-text search over actor, action and description"
//	@Param			actor_user_id	query	string	false	"Filter by acting user id"
//	@Param			action			query	string	false	"Filter by action code"
//	@Param			entity_type		query	string	false	"Filter by entity type"
//	@Param			entity_id		query	string	false	"Filter by entity id"
//	@Param			status			query	string	false	"Filter by status (SUCCESS, FAILURE, DENIED)"
//	@Param			from			query	string	false	"ISO-8601 lower bound on created_at"
//	@Param			to				query	string	false	"ISO-8601 upper bound on created_at"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/audit/logs [get]
func (h *Handler) List(c *gin.Context) {
	q := c.Request.URL.Query()
	params := auditdomain.ListParams{
		Pagination: platformdb.ParsePagination(q,
			map[string]string{"created_at": "created_at"},
			map[string]struct{}{},
		),
		ActorUserID: q.Get("actor_user_id"),
		Action:      q.Get("action"),
		EntityType:  q.Get("entity_type"),
		EntityID:    q.Get("entity_id"),
		Status:      q.Get("status"),
	}
	params.From = parseTime(q.Get("from"))
	params.To = parseTime(q.Get("to"))

	out, err := h.service.List(c.Request.Context(), params)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

func parseTime(raw string) *time.Time {
	if raw == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return &t
		}
	}
	return nil
}
