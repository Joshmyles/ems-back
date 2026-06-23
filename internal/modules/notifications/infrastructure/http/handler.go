package http

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	notifapp "dispatch/internal/modules/notifications/application"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *notifapp.Service
}

func NewHandler(service *notifapp.Service) *Handler {
	return &Handler{service: service}
}

// ListMy godoc
//
//	@Summary	List my notifications
//	@Tags		Notifications
//	@Produce	json
//	@Security	BearerAuth
//	@Param		page				query		int		false	"Page number"		default(1)
//	@Param		page_size			query		int		false	"Page size"			default(20)
//	@Param		filter[status]		query		string	false	"Filter by status"	Enums(PENDING,SENT,DELIVERED,READ,FAILED)
//	@Param		filter[channel]		query		string	false	"Filter by channel"	Enums(SMS,EMAIL,PUSH,IN_APP)
//	@Param		filter[date_from]	query		string	false	"Filter by created_at from (ISO 8601)"
//	@Param		filter[date_to]		query		string	false	"Filter by created_at to (ISO 8601)"
//	@Success	200					{object}	map[string]interface{}
//	@Failure	500					{object}	map[string]interface{}
//	@Router		/notifications [get]
func (h *Handler) ListMy(c *gin.Context) {
	userID := c.GetString("user_id")
	p := platformdb.ParsePagination(
		c.Request.URL.Query(),
		map[string]string{
			"created_at": "n.created_at",
		},
		map[string]struct{}{
			"status":    {},
			"channel":   {},
			"date_from": {},
			"date_to":   {},
		},
	)
	out, err := h.service.ListMy(c.Request.Context(), userID, p)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// Get godoc
//
//	@Summary	Get notification
//	@Tags		Notifications
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Notification ID"
//	@Success	200	{object}	map[string]interface{}
//	@Failure	404	{object}	map[string]interface{}
//	@Router		/notifications/{id} [get]
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
//	@Summary	Create notification
//	@Tags		Notifications
//	@Accept		json
//	@Produce	json
//	@Security	BearerAuth
//	@Param		payload	body		notifapp.CreateNotificationRequest	true	"Create notification payload"
//	@Success	201		{object}	map[string]interface{}
//	@Failure	400		{object}	map[string]interface{}
//	@Failure	500		{object}	map[string]interface{}
//	@Router		/notifications [post]
func (h *Handler) Create(c *gin.Context) {
	var req notifapp.CreateNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.Create(
		c.Request.Context(),
		req.Type, // string
		strings.ToUpper(strings.TrimSpace(req.Channel)), // string
		req.RecipientUserID,                             // *string
		req.RecipientPhone,                              // *string
		req.RecipientEmail,                              // *string
		req.Title,                                       // *string
		req.LinkedEntityType,                            // *string
		req.Body,                                        // string
		req.LinkedEntityID,                              // *string
	)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
}

// MarkRead godoc
//
//	@Summary	Mark notification as read
//	@Tags		Notifications
//	@Produce	json
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Notification ID"
//	@Success	200	{object}	map[string]interface{}
//	@Failure	500	{object}	map[string]interface{}
//	@Router		/notifications/{id}/read [post]
func (h *Handler) MarkRead(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	if err := h.service.UpdateStatusForUser(c.Request.Context(), id, userID, "READ"); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, gin.H{"message": "notification marked read"})
}
