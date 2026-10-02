package infrastructure

import (
	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"
)

// RegisterRoutes mounts the analytics endpoints. Every report is gated by
// reports.view — the permission the RBAC catalogue defines for reporting.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	rg.Use(rbacmiddleware.RequirePermission(rbacSvc, "reports.view"))

	rg.GET("/summary", h.GetSummary)
	rg.GET("/fuel", h.GetFuelAnalytics)

	rg.GET("/overview", h.GetOverview)
	rg.GET("/demand", h.GetDemand)
	rg.GET("/response", h.GetResponse)
	rg.GET("/referrals", h.GetReferrals)
	rg.GET("/geo", h.GetGeo)
	rg.GET("/clinical", h.GetClinical)
	rg.GET("/fleet", h.GetFleet)
	rg.GET("/data-quality", h.GetDataQuality)
	rg.GET("/exports/:dataset", h.ExportRegister)
}
