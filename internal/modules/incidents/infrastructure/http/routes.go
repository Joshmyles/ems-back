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
	secured.PUT("/:id", rbacmiddleware.RequirePermission(rbacSvc, "incidents.triage"), h.Update)
	secured.PATCH("/:id/status", rbacmiddleware.RequirePermission(rbacSvc, "incidents.triage"), h.UpdateStatus)
}
