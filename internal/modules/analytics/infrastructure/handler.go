package infrastructure

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	analyticsapp "dispatch/internal/modules/analytics/application"
	"dispatch/internal/platform/httpx"
	"dispatch/internal/shared/types"
)

type Handler struct {
	service *analyticsapp.Service
	logger  *zap.Logger
	audit   types.AuditRecorder
}

func NewHandler(service *analyticsapp.Service, logger *zap.Logger, audit types.AuditRecorder) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	if audit == nil {
		audit = types.NoopAuditRecorder{}
	}
	return &Handler{service: service, logger: logger, audit: audit}
}

// GetSummary godoc
// @Summary Consolidated analytics summary
// @Description Returns system-wide reporting: incident totals, assignments, referrals, patient transfers, outcome distribution and district-level breakdowns. Optionally filtered by date range and district.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Param date_from query string false "Start date YYYY-MM-DD"
// @Param date_to query string false "End date YYYY-MM-DD"
// @Param district_id query string false "District ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /analytics/summary [get]
func (h *Handler) GetSummary(c *gin.Context) {
	var q analyticsapp.SummaryQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.GetSummary(c.Request.Context(), q)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}

// GetFuelAnalytics godoc
// @Summary Fuel consumption analytics
// @Description Fleet-wide fuel consumption plus per-funding-source spend so funders can see how their fuel allocation was used.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /analytics/fuel [get]
func (h *Handler) GetFuelAnalytics(c *gin.Context) {
	var q analyticsapp.FuelQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := h.service.GetFuelAnalytics(c.Request.Context(), q)
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.OK(c, out)
}
