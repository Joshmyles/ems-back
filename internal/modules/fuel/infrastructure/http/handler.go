package http

import (
	"errors"
	"net/http"
	"strings"

	fuelapp "dispatch/internal/modules/fuel/application"
	"dispatch/internal/modules/fuel/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/gin-gonic/gin"
)

// driverScopeUserID returns the user ID to scope fuel-log reads to when the
// caller's only role is DRIVER. A driver should only see fuel logs for the
// ambulance they are the active driver on. Any non-DRIVER role grants the
// broader view and this returns nil.
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
	svc *fuelapp.Service
}

func NewHandler(svc *fuelapp.Service) *Handler {
	return &Handler{svc: svc}
}

// ListFuelLogs godoc
//
//	@Summary		List fuel logs
//	@Description	List fuel logs with pagination
//	@Tags			Fuel
//	@Security		BearerAuth
//	@Param			page					query		int		false	"Page number"	default(1)
//	@Param			page_size				query		int		false	"Page size"		default(20)
//	@Param			search					query		string	false	"Search query"
//	@Param			sort_by					query		string	false	"Sort by field"			default(created_at)
//	@Param			sort_order				query		string	false	"Sort order (ASC/DESC)"	default(DESC)
//	@Param			filter[ambulance_id]	query		string	false	"Filter by ambulance_id (UUID)"
//	@Param			filter[date_from]		query		string	false	"Filter by filled_at from (ISO 8601)"
//	@Param			filter[date_to]			query		string	false	"Filter by filled_at to (ISO 8601)"
//	@Success		200						{object}	platformdb.PageResult[domain.FuelLog]
//	@Failure		401						{object}	map[string]any
//	@Failure		403						{object}	map[string]any
//	@Failure		500						{object}	map[string]any
//	@Router			/fuel/logs [get]
func (h *Handler) List(c *gin.Context) {
	p := platformdb.ParsePagination(
		c.Request.URL.Query(),
		map[string]string{
			"created_at":  "fl.created_at",
			"filled_at":   "fl.filled_at",
			"liters":      "fl.liters",
			"cost":        "fl.cost",
			"odometer_km": "fl.odometer_km",
		},
		map[string]struct{}{
			"ambulance_id": {},
			"date_from":    {},
			"date_to":      {},
		},
	)

	items, total, err := h.svc.List(c.Request.Context(), p, driverScopeUserID(c))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to list fuel logs"})
		return
	}

	c.JSON(http.StatusOK, platformdb.PageResult[domain.FuelLog]{
		Items: items,
		Meta:  platformdb.NewPageMeta(p, total),
	})
}

// GetFuelLog godoc
//
//	@Summary	Get fuel log by id
//	@Tags		Fuel
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Fuel log ID (UUID)"
//	@Success	200	{object}	map[string]any
//	@Failure	401	{object}	map[string]any
//	@Failure	403	{object}	map[string]any
//	@Failure	404	{object}	map[string]any
//	@Failure	500	{object}	map[string]any
//	@Router		/fuel/logs/{id} [get]
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	item, err := h.svc.Get(c.Request.Context(), id, driverScopeUserID(c))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "fuel log not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// CreateFuelLog godoc
//
//	@Summary	Create fuel log
//	@Tags		Fuel
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		payload	body		fuelapp.CreateFuelLogRequest	true	"Fuel log payload"
//	@Success	201		{object}	map[string]any
//	@Failure	400		{object}	map[string]any
//	@Failure	401		{object}	map[string]any
//	@Failure	403		{object}	map[string]any
//	@Failure	500		{object}	map[string]any
//	@Router		/fuel/logs [post]
func (h *Handler) Create(c *gin.Context) {
	var req fuelapp.CreateFuelLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	var filledBy *string
	if v := c.GetString("user_id"); v != "" {
		filledBy = &v
	}
	item, err := h.svc.Create(c.Request.Context(), req, filledBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to create fuel log"})
		return
	}
	c.JSON(http.StatusCreated, item)
}

// UpdateFuelLog godoc
//
//	@Summary	Update fuel log
//	@Tags		Fuel
//	@Security	BearerAuth
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string							true	"Fuel log ID (UUID)"
//	@Param		payload	body		fuelapp.UpdateFuelLogRequest	true	"Update payload"
//	@Success	200		{object}	map[string]any
//	@Failure	400		{object}	map[string]any
//	@Failure	401		{object}	map[string]any
//	@Failure	403		{object}	map[string]any
//	@Failure	404		{object}	map[string]any
//	@Failure	500		{object}	map[string]any
//	@Router		/fuel/logs/{id} [put]
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req fuelapp.UpdateFuelLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	item, err := h.svc.Update(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "fuel log not found"})
		return
	}
	c.JSON(http.StatusOK, item)
}

// DeleteFuelLog godoc
//
//	@Summary	Delete fuel log
//	@Tags		Fuel
//	@Security	BearerAuth
//	@Param		id	path		string	true	"Fuel log ID (UUID)"
//	@Success	204	{object}	nil
//	@Failure	401	{object}	map[string]any
//	@Failure	403	{object}	map[string]any
//	@Failure	404	{object}	map[string]any
//	@Failure	500	{object}	map[string]any
//	@Router		/fuel/logs/{id} [delete]
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "fuel log not found"})
		return
	}
	c.Status(http.StatusNoContent)
}

// GetPublicFuelLog godoc
//
//	@Summary		Get public fuel log by QR token
//	@Description	Public, unauthenticated view returned when a fuel log QR code is scanned
//	@Tags			Fuel
//	@Param			token	path		string	true	"Fuel log public token"
//	@Success		200		{object}	domain.FuelLogPublicView
//	@Failure		404		{object}	map[string]any
//	@Router			/public/fuel-logs/{token} [get]
func (h *Handler) GetPublic(c *gin.Context) {
	token := c.Param("token")
	view, err := h.svc.GetPublic(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "fuel log not found"})
		return
	}
	c.JSON(http.StatusOK, view)
}

// ConfirmPublicFuelLog godoc
//
//	@Summary		Confirm fuel dispense from the QR page
//	@Description	Public endpoint used by the fuel station attendant to confirm fuel was dispensed
//	@Tags			Fuel
//	@Accept			json
//	@Produce		json
//	@Param			token	path		string							true	"Fuel log public token"
//	@Param			payload	body		fuelapp.ConfirmFuelDispenseRequest	true	"Confirmation payload"
//	@Success		200		{object}	domain.FuelLogPublicView
//	@Failure		400		{object}	map[string]any
//	@Failure		404		{object}	map[string]any
//	@Failure		409		{object}	map[string]any
//	@Router			/public/fuel-logs/{token}/confirm [post]
func (h *Handler) ConfirmPublic(c *gin.Context) {
	token := c.Param("token")
	var req fuelapp.ConfirmFuelDispenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "attendant name is required"})
		return
	}

	view, err := h.svc.ConfirmDispense(c.Request.Context(), token, req)
	if err != nil {
		switch {
		case errors.Is(err, fuelapp.ErrFuelLogNotFound):
			c.JSON(http.StatusNotFound, gin.H{"message": "fuel log not found"})
		case errors.Is(err, fuelapp.ErrAlreadyConfirmed):
			c.JSON(http.StatusConflict, gin.H{"message": "this fuel log has already been confirmed"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"message": "failed to confirm fuel dispense"})
		}
		return
	}
	c.JSON(http.StatusOK, view)
}
