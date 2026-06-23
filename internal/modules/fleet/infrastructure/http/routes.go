package http

import (
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	rbacapp "dispatch/internal/modules/rbac/application"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	rg.GET("", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "fleet.read", "DRIVER"), h.List)
	rg.GET("/:id", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "fleet.read", "DRIVER"), h.Get)
	rg.POST("", rbacmiddleware.RequirePermission(rbacSvc, "fleet.manage"), h.Create)
	rg.PUT("/:id", rbacmiddleware.RequirePermission(rbacSvc, "fleet.manage"), h.Update)
	rg.DELETE("/:id", rbacmiddleware.RequirePermission(rbacSvc, "fleet.manage"), h.Delete)
	rg.POST("/:id/driver", rbacmiddleware.RequirePermission(rbacSvc, "fleet.manage"), h.AssignDriver)
	rg.DELETE("/:id/driver", rbacmiddleware.RequirePermission(rbacSvc, "fleet.manage"), h.UnassignDriver)
}
