package http

import (
	"net/http"

	"github.com/gin-gonic/gin"

	bloodapp "dispatch/internal/modules/blood/application"
	"dispatch/internal/modules/blood/application/dto"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/httpx"
)

type Handler struct{ service *bloodapp.Service }

func NewHandler(service *bloodapp.Service) *Handler { return &Handler{service: service} }

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
	if req.RequestedByUserID == nil {
		if uid := c.GetString("user_id"); uid != "" {
			req.RequestedByUserID = &uid
		}
	}
	out, err := h.service.RaiseRequisition(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
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
	out, err := h.service.ListRequisitions(c.Request.Context(), p)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
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
	out, err := h.service.CreateOffer(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.Created(c, out)
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
		httpx.Error(c, http.StatusInternalServerError, err.Error())
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
	if err := h.service.AcceptOffer(c.Request.Context(), requisitionID, offerID, body.ActorUserID); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
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
//	@Router			/pickup-assignments [post]
func (h *Handler) AssignPickup(c *gin.Context) {
	var req dto.AssignBloodPickupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.AssignPickup(c.Request.Context(), req)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
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
//	@Router			/pickup-assignments/{assignmentId}/collect [post]
func (h *Handler) MarkCollected(c *gin.Context) {
	assignmentID := c.Param("assignmentId")
	var body dto.MarkCollectedRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.MarkCollected(c.Request.Context(), assignmentID, body.BloodRequisitionID, body.ActorUserID); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
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
//	@Router			/pickup-assignments/{assignmentId}/deliver [post]
func (h *Handler) MarkDelivered(c *gin.Context) {
	assignmentID := c.Param("assignmentId")
	var body dto.MarkDeliveredRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.service.MarkDelivered(c.Request.Context(), assignmentID, body.BloodRequisitionID, body.ActorUserID); err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, gin.H{"message": "blood marked delivered"})
}
