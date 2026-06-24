package http

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	// Responders are a read-only projection of fleet data, so they reuse the
	// fleet.read permission (DRIVER role also permitted, mirroring fleet).
	rg.GET("", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "fleet.read", "DRIVER"), h.List)
	rg.GET("/:id", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "fleet.read", "DRIVER"), h.Get)
}
