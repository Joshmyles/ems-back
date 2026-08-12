package http

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	// MEDIC: field medics raise blood requisitions from the mobile app.
	rg.POST("/requisitions", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "incidents.create", "MEDIC"), h.RaiseRequisition)
	rg.GET("/requisitions", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "incidents.read", "MEDIC"), h.ListRequisitions)
	rg.PUT("/requisitions/:id", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "incidents.create", "MEDIC"), h.UpdateRequisition)
	rg.DELETE("/requisitions/:id", rbacmiddleware.RequirePermissionOrRole(rbacSvc, "incidents.create", "MEDIC"), h.DeleteRequisition)
	rg.POST("/requisitions/:id/broadcast", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.Broadcast)
	rg.PATCH("/requisitions/:id/decision", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.DecideRequisition)
	rg.GET("/requisitions/:id/offers", rbacmiddleware.RequirePermission(rbacSvc, "incidents.read"), h.ListOffers)
	rg.POST("/offers", rbacmiddleware.RequirePermission(rbacSvc, "incidents.create"), h.CreateOffer)
	rg.POST("/requisitions/:id/offers/:offerId/accept", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.AcceptOffer)
	rg.POST("/pickup-assignments", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.AssignPickup)
	rg.POST("/pickup-assignments/:assignmentId/collect", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.update_status"), h.MarkCollected)
	rg.POST("/pickup-assignments/:assignmentId/deliver", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.update_status"), h.MarkDelivered)
}
