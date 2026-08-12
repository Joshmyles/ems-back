package http

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	rg.GET("/logs", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "fuel.read", "DRIVER"), h.List)
	rg.GET("/logs/:id", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "fuel.read", "DRIVER"), h.Get)
	rg.POST("/logs", rbacmiddleware.RequirePermission(rbacSvc, "fuel.manage"), h.Create)
	rg.PUT("/logs/:id", rbacmiddleware.RequirePermission(rbacSvc, "fuel.manage"), h.Update)
	rg.DELETE("/logs/:id", rbacmiddleware.RequirePermission(rbacSvc, "fuel.manage"), h.Delete)

	rg.GET("/funding-sources", rbacmiddleware.RequirePermission(rbacSvc, "fuel.read"), h.ListFundingSources)
	rg.POST("/funding-sources", rbacmiddleware.RequirePermission(rbacSvc, "fuel.manage"), h.CreateFundingSource)
	rg.DELETE("/funding-sources/:id", rbacmiddleware.RequirePermission(rbacSvc, "fuel.manage"), h.DeleteFundingSource)
}

// RegisterPublicRoutes wires the unauthenticated QR-scan endpoints. These are
// mounted outside the auth-protected group so a fuel station attendant can
// open the link without an account.
func RegisterPublicRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("/fuel-logs/:token", h.GetPublic)
	rg.POST("/fuel-logs/:token/confirm", h.ConfirmPublic)
}
