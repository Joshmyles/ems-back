package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	bloodapp "dispatch/internal/modules/blood/application"
	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/modules/blood/application/dto"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *bloodapp.Service
	rbac    *rbacapp.Service
}

func NewHandler(service *bloodapp.Service, rbac *rbacapp.Service) *Handler {
	return &Handler{service: service, rbac: rbac}
}

// isPrivileged reports whether the caller holds dispatch.assign — the
// dispatcher/admin capability that allows managing any requisition. Field
// medics without it may only touch their own.
func (h *Handler) isPrivileged(c *gin.Context) bool {
	uid := c.GetString("user_id")
	if uid == "" || h.rbac == nil {
		return false
	}
	ok, err := h.rbac.HasPermission(c.Request.Context(), uid, "dispatch.assign", "GLOBAL", nil)
	return err == nil && ok
}

// writeError maps service and database errors to HTTP statuses so client
// mistakes surface as 4xx with a readable message instead of a bare 500.
func writeError(c *gin.Context, err error) {
	var pgErr *pgconn.PgError
	switch {
	case errors.Is(err, bloodapp.ErrInvalidInput):
		httpx.Error(c, http.StatusBadRequest, err.Error())
	case errors.Is(err, bloodapp.ErrNotFound):
		httpx.Error(c, http.StatusNotFound, err.Error())
	case errors.Is(err, bloodapp.ErrRequisitionForbidden):
		httpx.Error(c, http.StatusForbidden, err.Error())
	case errors.Is(err, bloodapp.ErrRequisitionLocked), errors.Is(err, bloodapp.ErrDecisionNotAllowed),
		errors.Is(err, bloodapp.ErrOfferNotOpen), errors.Is(err, bloodapp.ErrAcceptNotAllowed):
		httpx.Error(c, http.StatusConflict, err.Error())
	case errors.As(err, &pgErr) && (pgErr.Code == "22P02" || pgErr.Code == "23514"):
		// invalid_text_representation (e.g. malformed UUID) / check_violation
		httpx.Error(c, http.StatusBadRequest, pgErr.Message)
	default:
		httpx.DBError(c, err)
	}
}

// stampActor fills an optional user-ID field from the auth context when the
// client left it out or sent "".
func stampActor(c *gin.Context, field **string) {
	if *field != nil && strings.TrimSpace(**field) != "" {
		return
	}
	if uid := c.GetString("user_id"); uid != "" {
		*field = &uid
	}
}

// RaiseRequisition godoc
//
//	@Summary		Raise blood requisition
//	@Description	Creates a new blood requisition request
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dto.CreateBloodRequisitionRequest	true	"Requisition details"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/blood/requisitions [post]
func (h *Handler) RaiseRequisition(c *gin.Context) {
	var req dto.CreateBloodRequisitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	// Stamp the requester from the auth context so "my requests" filtering
	// works without trusting the client to identify itself.
	stampActor(c, &req.RequestedByUserID)
	out, err := h.service.RaiseRequisition(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.Created(c, out)
}

func (h *Handler) Broadcast(c *gin.Context) {
	id := c.Param("id")
	var payload struct {
		DestinationLat *float64 `json:"destination_lat"`
		DestinationLon *float64 `json:"destination_lon"`
	}
	_ = c.ShouldBindJSON(&payload)
	out, err := h.service.BroadcastRequisition(c.Request.Context(), id, payload.DestinationLat, payload.DestinationLon)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, out)
}

// ListRequisitions godoc
//
//	@Summary		List blood requisitions
//	@Description	Returns paginated blood requisitions with search, sorting, and filters
//	@Tags			Blood
//	@Produce		json
//	@Security		BearerAuth
//	@Param			page				query		int		false	"Page number"	default(1)
//	@Param			page_size			query		int		false	"Page size"		default(20)
//	@Param			search				query		string	false	"Search term"
//	@Param			status				query		string	false	"Filter by status"	Enums(OPEN,BROADCASTING,MATCHED,PICKUP_ASSIGNED,COLLECTED,DELIVERED,CANCELLED,EXPIRED)
//	@Param			blood_type			query		string	false	"Filter by blood type"
//	@Param			urgency_level		query		string	false	"Filter by urgency level"	Enums(EMERGENCY,URGENT,ROUTINE)
//	@Param			filter[date_from]	query		string	false	"Filter by created_at from (ISO 8601)"
//	@Param			filter[date_to]		query		string	false	"Filter by created_at to (ISO 8601)"
//	@Param			sort_by				query		string	false	"Sort by field: created_at, status, urgency_level"
//	@Param			sort_order			query		string	false	"Sort order: asc or desc"
//	@Success		200					{object}	map[string]interface{}
//	@Failure		500					{object}	map[string]interface{}
//	@Router			/blood/requisitions [get]
func (h *Handler) ListRequisitions(c *gin.Context) {
	p := platformdb.ParsePagination(c.Request.URL.Query(), map[string]string{
		"created_at":      "br.created_at",
		"status":          "br.status",
		"urgency_level":   "br.urgency_level",
		"units_requested": "br.units_requested",
	}, map[string]struct{}{
		"status":               {},
		"urgency_level":        {},
		"date_from":            {},
		"date_to":              {},
		"requested_by_user_id": {},
	})
	// Also honour the plain ?status= / ?urgency_level= form documented above.
	query := c.Request.URL.Query()
	for _, key := range []string{"status", "urgency_level", "requested_by_user_id"} {
		if v := strings.TrimSpace(query.Get(key)); v != "" && p.Filters[key] == "" {
			p.Filters[key] = v
		}
	}
	out, err := h.service.ListRequisitions(c.Request.Context(), p)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, out)
}

// CreateOffer godoc
//
//	@Summary		Create blood offer
//	@Description	Creates a new blood offer for a requisition
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dto.CreateBloodOfferRequest	true	"Offer details"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/blood/offers [post]
func (h *Handler) CreateOffer(c *gin.Context) {
	var req dto.CreateBloodOfferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	stampActor(c, &req.OfferedByUserID)
	out, err := h.service.CreateOffer(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.Created(c, out)
}

// SummarizeRequisitions godoc
//
//	@Summary		Summarize blood requisitions
//	@Description	Returns requisition counts by status and urgency across all records
//	@Tags			Blood
//	@Produce		json
//	@Security		BearerAuth
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/blood/requisitions/summary [get]
func (h *Handler) SummarizeRequisitions(c *gin.Context) {
	out, err := h.service.SummarizeRequisitions(c.Request.Context())
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, out)
}

// GetRequisitionTracking godoc
//
//	@Summary		Track blood requisition
//	@Description	Returns the status history and live pickup assignment for a requisition
//	@Tags			Blood
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Blood Requisition ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		404	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/blood/requisitions/{id}/tracking [get]
func (h *Handler) GetRequisitionTracking(c *gin.Context) {
	out, err := h.service.GetRequisitionTracking(c.Request.Context(), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, out)
}

// ListOffers godoc
//
//	@Summary		List offers for requisition
//	@Description	Returns the list of blood offers for a given requisition
//	@Tags			Blood
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path		string	true	"Blood Requisition ID"
//	@Success		200	{object}	map[string]interface{}
//	@Failure		500	{object}	map[string]interface{}
//	@Router			/blood/requisitions/{id}/offers [get]
func (h *Handler) ListOffers(c *gin.Context) {
	requisitionID := c.Param("id")
	p := platformdb.ParsePagination(c.Request.URL.Query(), map[string]string{
		"created_at":    "bro.created_at",
		"status":        "bro.status",
		"units_offered": "bro.units_offered",
	}, map[string]struct{}{
		"status": {},
	})
	out, err := h.service.ListOffers(c.Request.Context(), requisitionID, p)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, out)
}

// AcceptOffer godoc
//
//	@Summary		Accept blood offer
//	@Description	Accepts a blood offer for a requisition and marks it as accepted
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string					true	"Blood Requisition ID"
//	@Param			offerId	path		string					true	"Blood Offer ID"
//	@Param			payload	body		dto.AcceptOfferRequest	false	"Optional audit actor"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/blood/requisitions/{id}/offers/{offerId}/accept [post]
func (h *Handler) AcceptOffer(c *gin.Context) {
	requisitionID := c.Param("id")
	offerID := c.Param("offerId")
	var body dto.AcceptOfferRequest
	_ = c.ShouldBindJSON(&body)
	stampActor(c, &body.ActorUserID)
	if err := h.service.AcceptOffer(c.Request.Context(), requisitionID, offerID, body.ActorUserID); err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "offer accepted"})
}

// AssignPickup godoc
//
//	@Summary		Assign blood pickup
//	@Description	Assigns a pickup for an accepted blood offer, creating a new assignment record
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			payload	body		dto.AssignBloodPickupRequest	true	"Pickup assignment details"
//	@Success		201		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		500		{object}	map[string]interface{}
//	@Router			/blood/pickup-assignments [post]
func (h *Handler) AssignPickup(c *gin.Context) {
	var req dto.AssignBloodPickupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	stampActor(c, &req.AssignedByUserID)
	out, err := h.service.AssignPickup(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.Created(c, out)
}

// MarkCollected godoc
//
//	@Summary		Mark blood as collected
//	@Description	Marks a blood pickup assignment as collected by the assigned user
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			assignmentId	path		string						true	"Pickup Assignment ID"
//	@Param			payload			body		dto.MarkCollectedRequest	true	"Payload with requisition ID and optional actor user ID"
//	@Success		200				{object}	map[string]interface{}
//	@Failure		400				{object}	map[string]interface{}
//	@Failure		500				{object}	map[string]interface{}
//	@Router			/blood/pickup-assignments/{assignmentId}/collect [post]
func (h *Handler) MarkCollected(c *gin.Context) {
	assignmentID := c.Param("assignmentId")
	var body dto.MarkCollectedRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	stampActor(c, &body.ActorUserID)
	if err := h.service.MarkCollected(c.Request.Context(), assignmentID, body.BloodRequisitionID, body.ActorUserID); err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "blood marked collected"})
}

// MarkDelivered godoc
//
//	@Summary		Mark blood as delivered
//	@Description	Marks a blood pickup assignment as delivered, completing the pickup process
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			assignmentId	path		string						true	"Pickup Assignment ID"
//	@Param			payload			body		dto.MarkDeliveredRequest	true	"Payload with requisition ID and optional actor user ID"
//	@Success		200				{object}	map[string]interface{}
//	@Failure		400				{object}	map[string]interface{}
//	@Failure		500				{object}	map[string]interface{}
//	@Router			/blood/pickup-assignments/{assignmentId}/deliver [post]
func (h *Handler) MarkDelivered(c *gin.Context) {
	assignmentID := c.Param("assignmentId")
	var body dto.MarkDeliveredRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	stampActor(c, &body.ActorUserID)
	if err := h.service.MarkDelivered(c.Request.Context(), assignmentID, body.BloodRequisitionID, body.ActorUserID); err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, gin.H{"message": "blood marked delivered"})
}


// UpdateRequisition godoc
//
//	@Summary		Update blood requisition
//	@Description	Edits a requisition. Medics may only edit their own while it is still OPEN/BROADCASTING; dispatchers/admins may edit any.
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string								true	"Requisition ID"
//	@Param			payload	body		dto.UpdateBloodRequisitionRequest	true	"Fields to update"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		403		{object}	map[string]interface{}
//	@Failure		409		{object}	map[string]interface{}
//	@Router			/blood/requisitions/{id} [put]
func (h *Handler) UpdateRequisition(c *gin.Context) {
	var req dto.UpdateBloodRequisitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.UpdateRequisition(
		c.Request.Context(), c.Param("id"), req,
		c.GetString("user_id"), h.isPrivileged(c),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, out)
}

// DeleteRequisition godoc
//
//	@Summary		Delete blood requisition
//	@Description	Deletes a requisition. Medics may only delete their own; blocked once a pickup workflow has started.
//	@Tags			Blood
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id	path	string	true	"Requisition ID"
//	@Success		204	"No Content"
//	@Failure		403	{object}	map[string]interface{}
//	@Failure		409	{object}	map[string]interface{}
//	@Router			/blood/requisitions/{id} [delete]
func (h *Handler) DeleteRequisition(c *gin.Context) {
	err := h.service.DeleteRequisition(
		c.Request.Context(), c.Param("id"),
		c.GetString("user_id"), h.isPrivileged(c),
	)
	if err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// DecideRequisition godoc
//
//	@Summary		Approve or decline a blood requisition
//	@Description	Records the admin/dispatcher decision. Allowed while the requisition is OPEN or BROADCASTING.
//	@Tags			Blood
//	@Accept			json
//	@Produce		json
//	@Security		BearerAuth
//	@Param			id		path		string	true	"Requisition ID"
//	@Param			payload	body		object	true	"{\"decision\": \"APPROVED|DECLINED\", \"notes\": \"optional\"}"
//	@Success		200		{object}	map[string]interface{}
//	@Failure		400		{object}	map[string]interface{}
//	@Failure		409		{object}	map[string]interface{}
//	@Router			/blood/requisitions/{id}/decision [patch]
func (h *Handler) DecideRequisition(c *gin.Context) {
	var payload struct {
		Decision string `json:"decision" binding:"required,oneof=APPROVED DECLINED approved declined"`
		Notes    string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		httpx.Error(c, http.StatusBadRequest, "decision must be APPROVED or DECLINED")
		return
	}
	var actor *string
	if uid := c.GetString("user_id"); uid != "" {
		actor = &uid
	}
	out, err := h.service.DecideRequisition(c.Request.Context(), c.Param("id"), payload.Decision, actor, payload.Notes)
	if err != nil {
		writeError(c, err)
		return
	}
	httpx.OK(c, out)
}
