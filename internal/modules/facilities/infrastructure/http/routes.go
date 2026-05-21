package http

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	rg.GET("", rbacmiddleware.RequirePermission(rbacSvc, "facilities.read"), h.List)
	rg.GET("/:uid", rbacmiddleware.RequirePermission(rbacSvc, "facilities.read"), h.Get)
	rg.POST("", rbacmiddleware.RequirePermission(rbacSvc, "facilities.manage"), h.Create)
	rg.PUT("/:uid", rbacmiddleware.RequirePermission(rbacSvc, "facilities.manage"), h.Update)
	rg.DELETE("/:uid", rbacmiddleware.RequirePermission(rbacSvc, "facilities.manage"), h.Delete)
	rg.PUT("/:uid/focal-person", rbacmiddleware.RequirePermission(rbacSvc, "facilities.manage"), h.SetFocalPerson)
	rg.DELETE("/:uid/focal-person", rbacmiddleware.RequirePermission(rbacSvc, "facilities.manage"), h.ClearFocalPerson)
}
