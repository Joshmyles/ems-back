package http

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	rg.GET("", rbacmiddleware.RequirePermission(rbacSvc, "trips.read"), h.List)
	rg.GET("/:id", rbacmiddleware.RequirePermission(rbacSvc, "trips.read"), h.Get)
	rg.POST("", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "trips.manage", "DRIVER", "DISPATCHER"), h.Create)
	rg.PUT("/:id", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "trips.manage", "DRIVER", "DISPATCHER"), h.Update)
	rg.PATCH("/:id", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "trips.manage", "DRIVER", "DISPATCHER"), h.Update)
	rg.DELETE("/:id", rbacmiddleware.RequirePermission(rbacSvc, "trips.manage"), h.Delete)
	rg.GET("/:id/events", rbacmiddleware.RequirePermission(rbacSvc, "trips.read"), h.ListEvents)
	rg.POST("/:id/events", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "trips.manage", "DRIVER", "DISPATCHER"), h.CreateEvent)
}
