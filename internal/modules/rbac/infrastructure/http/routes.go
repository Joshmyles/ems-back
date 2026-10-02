package http

import (
	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	rg.GET("/me/permissions", h.MyPermissions)

	read := rbacmiddleware.RequirePermission(rbacSvc, "roles.read")
	manage := rbacmiddleware.RequirePermission(rbacSvc, "roles.manage")

	rg.GET("/permissions", read, h.ListPermissions)
	rg.GET("/roles", read, h.ListRoles)
	rg.GET("/roles/:id", read, h.GetRole)
	rg.POST("/roles", manage, h.CreateRole)
	rg.PUT("/roles/:id", manage, h.UpdateRole)
	rg.DELETE("/roles/:id", manage, h.DeleteRole)
	rg.PUT("/roles/:id/permissions", manage, h.SetRolePermissions)
}
