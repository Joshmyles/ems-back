package http

import (
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service, authMiddleware gin.HandlerFunc) {

	rg.POST("", h.Create)
	secured := rg.Group("")
	secured.Use(authMiddleware)

	secured.GET("", rbacmiddleware.RequirePermission(rbacSvc, "incidents.read"), h.List)
	secured.GET("/:id", rbacmiddleware.RequirePermission(rbacSvc, "incidents.read"), h.GetByID)
	secured.GET("/:id/triage", rbacmiddleware.RequirePermission(rbacSvc, "incidents.read"), h.GetTriage)
	secured.PUT("/:id", rbacmiddleware.RequirePermission(rbacSvc, "incidents.triage"), h.Update)
	secured.DELETE("/:id", rbacmiddleware.RequirePermission(rbacSvc, "incidents.delete"), h.Delete)
	secured.PATCH("/:id/status", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "incidents.triage", "DRIVER", "MEDIC"), h.UpdateStatus)
	secured.POST("/:id/feedback", rbacmiddleware.RequirePermission(rbacSvc, "incidents.feedback"), h.CreateFeedback)
	secured.GET("/:id/feedback", rbacmiddleware.RequirePermission(rbacSvc, "incidents.read"), h.ListFeedback)
}

// secured.PATCH("/:id/status", rbacmiddleware.RequirePermission(rbacSvc, "incidents.triage"), h.UpdateStatus)