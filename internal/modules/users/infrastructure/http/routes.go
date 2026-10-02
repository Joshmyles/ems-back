package http

import (
	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	rg.POST("", rbacmiddleware.RequirePermission(rbacSvc, "users.create"), h.Create)
	rg.GET("", rbacmiddleware.RequirePermission(rbacSvc, "users.read"), h.List)
	rg.GET("/:id", rbacmiddleware.RequirePermission(rbacSvc, "users.read"), h.GetByID)
	rg.PUT("/:id", rbacmiddleware.RequirePermission(rbacSvc, "users.update"), h.Update)
	rg.DELETE("/:id", rbacmiddleware.RequirePermission(rbacSvc, "users.delete"), h.Delete)

	// Activate / deactivate replace the old lock/unlock flow.
	rg.POST("/:id/activate", rbacmiddleware.RequirePermission(rbacSvc, "users.update"), h.Activate)
	rg.POST("/:id/deactivate", rbacmiddleware.RequirePermission(rbacSvc, "users.update"), h.Deactivate)

	// Self-service OR an administrator with users.update. The handler/service
	// additionally forbid an admin-style reset (reset_by_admin / another user)
	// unless the caller actually holds users.update.
	rg.POST("/:id/change-password", rbacmiddleware.RequireSelfOrPermission(rbacSvc, "users.update"), h.ChangePassword)
	rg.PATCH("/:id/profile", rbacmiddleware.RequireSelfOrPermission(rbacSvc, "users.update"), h.UpdateProfile)
	rg.GET("/:id/details", rbacmiddleware.RequireSelfOrPermission(rbacSvc, "users.read"), h.GetDetails)

	// Role membership is a privileged operation — gated by roles.manage and, in
	// the handler, by the privilege-escalation guard.
	rg.POST("/:id/roles", rbacmiddleware.RequirePermission(rbacSvc, "roles.manage"), h.AssignRole)
	rg.DELETE("/:id/roles/:roleId", rbacmiddleware.RequirePermission(rbacSvc, "roles.manage"), h.RemoveRole)

	// Org-scope and capability assignment require users.update.
	rg.POST("/:id/assignments", rbacmiddleware.RequirePermission(rbacSvc, "users.update"), h.AssignUser)
	rg.PATCH("/assignments/:assignmentId", rbacmiddleware.RequirePermission(rbacSvc, "users.update"), h.UpdateAssignment)
	rg.POST("/:id/capabilities", rbacmiddleware.RequirePermission(rbacSvc, "users.update"), h.AssignCapability)
	rg.PATCH("/capabilities/:capabilityRecordId", rbacmiddleware.RequirePermission(rbacSvc, "users.update"), h.UpdateCapability)
}
