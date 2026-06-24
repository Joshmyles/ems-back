package infrastructure

import (
	"net/http"

	"github.com/gin-gonic/gin"

	analyticsapp "dispatch/internal/modules/analytics/application"
	"dispatch/internal/platform/httpx"
)

type Handler struct {
	service *analyticsapp.Service
}

func NewHandler(service *analyticsapp.Service) *Handler { return &Handler{service: service} }

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
