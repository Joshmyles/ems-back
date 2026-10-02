package infrastructure

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	analyticsapp "dispatch/internal/modules/analytics/application"
	"dispatch/internal/platform/httpx"
	"dispatch/internal/shared/types"
)

// respond binds the shared report query, runs the report and maps errors:
// filter problems are the client's (400); anything else is logged and hidden
// behind a generic 500 so SQL details never reach the browser.
func respond[T any](h *Handler, c *gin.Context, name string, fn func(context.Context, analyticsapp.ReportQuery) (T, error)) {
	var q analyticsapp.ReportQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	out, err := fn(c.Request.Context(), q)
	if err != nil {
		if errors.Is(err, analyticsapp.ErrInvalidReportQuery) {
			httpx.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		h.logger.Error("analytics report failed", zap.String("report", name), zap.Error(err))
		httpx.Error(c, http.StatusInternalServerError, "could not build the "+name+" report")
		return
	}
	httpx.OK(c, out)
}

// GetOverview godoc
// @Summary Executive overview report
// @Description Headline KPIs against the previous period, the bucketed incident trend, distributions and rule-generated insights.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Param date_from query string false "Start date YYYY-MM-DD (omit for all time)"
// @Param date_to query string false "End date YYYY-MM-DD (inclusive)"
// @Param district_id query string false "District ID"
// @Param granularity query string false "day | week | month (default: auto)"
// @Success 200 {object} map[string]interface{}
// @Router /analytics/overview [get]
func (h *Handler) GetOverview(c *gin.Context) { respond(h, c, "overview", h.service.Overview) }

// GetDemand godoc
// @Summary Demand patterns report
// @Description Hour-of-day by weekday heatmap, daily calendar, priority mix by hour and a 14-day projection.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/demand [get]
func (h *Handler) GetDemand(c *gin.Context) { respond(h, c, "demand", h.service.Demand) }

// GetResponse godoc
// @Summary Response-time performance report
// @Description Milestone funnel, dispatch/scene/closure percentiles, SLA compliance by priority and channel, per-period distribution, dispatcher performance and the slowest cases.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/response [get]
func (h *Handler) GetResponse(c *gin.Context) { respond(h, c, "response", h.service.Response) }

// GetReferrals godoc
// @Summary Referral network report
// @Description Facility-to-facility transfer flows, level-of-care escalation, busiest facilities and referral reasons.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/referrals [get]
func (h *Handler) GetReferrals(c *gin.Context) { respond(h, c, "referrals", h.service.Referrals) }

// GetGeo godoc
// @Summary Map layers
// @Description Incident points with valid coordinates, referral facilities and the flows between them.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/geo [get]
func (h *Handler) GetGeo(c *gin.Context) { respond(h, c, "map", h.service.Geo) }

// GetClinical godoc
// @Summary Clinical profile report
// @Description Triage red flags, parsed vital signs, age/sex pyramid, case mix and maternal & neonatal indicators.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/clinical [get]
func (h *Handler) GetClinical(c *gin.Context) { respond(h, c, "clinical", h.service.Clinical) }

// GetFleet godoc
// @Summary Fleet & fuel integrity report
// @Description Ambulance utilisation, workload concentration, crew workload, fuel efficiency and fuel-log integrity anomalies.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/fleet [get]
func (h *Handler) GetFleet(c *gin.Context) { respond(h, c, "fleet", h.service.Fleet) }

// GetDataQuality godoc
// @Summary Data quality & adoption report
// @Description Weighted completeness score, consistency issues, capture-rate trend, quality by channel and system adoption.
// @Tags Analytics
// @Produce json
// @Security BearerAuth
// @Router /analytics/data-quality [get]
func (h *Handler) GetDataQuality(c *gin.Context) {
	respond(h, c, "data quality", h.service.DataQuality)
}

// lazyCSVWriter defers the CSV headers until the first byte is written, so a
// query that fails before producing output can still return a JSON error.
type lazyCSVWriter struct {
	c        *gin.Context
	filename string
	started  bool
}

func (w *lazyCSVWriter) Write(p []byte) (int, error) {
	if !w.started {
		w.started = true
		w.c.Header("Content-Type", "text/csv; charset=utf-8")
		w.c.Header("Content-Disposition", `attachment; filename="`+w.filename+`"`)
		w.c.Header("Cache-Control", "no-store")
		w.c.Status(http.StatusOK)
	}
	return w.c.Writer.Write(p)
}

// ExportRegister godoc
// @Summary Export a register as CSV
// @Description Streams the incident, dispatch, fuel or referral register for the window. Caller and patient names and phone numbers are excluded.
// @Tags Analytics
// @Produce text/csv
// @Security BearerAuth
// @Param dataset path string true "incidents | dispatches | fuel | referrals"
// @Router /analytics/exports/{dataset} [get]
func (h *Handler) ExportRegister(c *gin.Context) {
	dataset := strings.ToLower(c.Param("dataset"))
	var q analyticsapp.ReportQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		httpx.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	f, err := h.service.PrepareExport(c.Request.Context(), dataset, q)
	if err != nil {
		if errors.Is(err, analyticsapp.ErrInvalidReportQuery) {
			httpx.Error(c, http.StatusBadRequest, err.Error())
			return
		}
		h.logger.Error("analytics export failed", zap.String("dataset", dataset), zap.Error(err))
		httpx.Error(c, http.StatusInternalServerError, "could not prepare the export")
		return
	}

	w := &lazyCSVWriter{c: c, filename: analyticsapp.ExportFilename(dataset, f)}
	exportErr := h.service.Export(c.Request.Context(), f, dataset, w)

	var roles []string
	if v, ok := c.Get("roles"); ok {
		roles, _ = v.([]string)
	}
	status := types.AuditStatusSuccess
	if exportErr != nil {
		status = types.AuditStatusFailure
	}
	h.audit.Record(c.Request.Context(), types.AuditEntry{
		ActorUserID:   c.GetString("user_id"),
		ActorUsername: c.GetString("username"),
		ActorRoles:    roles,
		Action:        types.AuditReportExport,
		EntityType:    "report",
		Status:        status,
		Description:   "Exported the " + dataset + " register",
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
		Metadata: map[string]any{
			"dataset":     dataset,
			"date_from":   q.DateFrom,
			"date_to":     q.DateTo,
			"district_id": q.DistrictID,
		},
	})

	if exportErr != nil {
		h.logger.Error("analytics export failed", zap.String("dataset", dataset), zap.Error(exportErr))
		if !w.started {
			httpx.Error(c, http.StatusInternalServerError, "could not export the register")
		}
	}
}
