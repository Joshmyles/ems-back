package http

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, rbacSvc *rbacapp.Service) {
	// Any authenticated user (the parent group already enforces a valid JWT)
	// may raise, list, update and delete blood requisitions. Ownership of
	// updates/deletes is still enforced inside the handler.
	rg.POST("/requisitions", h.RaiseRequisition)
	rg.GET("/requisitions", h.ListRequisitions)
	rg.PUT("/requisitions/:id", h.UpdateRequisition)
	rg.DELETE("/requisitions/:id", h.DeleteRequisition)
	rg.POST("/requisitions/:id/broadcast", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.Broadcast)
	rg.PATCH("/requisitions/:id/decision", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.DecideRequisition)
	rg.GET("/requisitions/:id/offers", rbacmiddleware.RequirePermission(rbacSvc, "incidents.read"), h.ListOffers)
	rg.POST("/offers", rbacmiddleware.RequirePermission(rbacSvc, "incidents.create"), h.CreateOffer)
	rg.POST("/requisitions/:id/offers/:offerId/accept", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.AcceptOffer)
	rg.POST("/pickup-assignments", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.assign"), h.AssignPickup)
	rg.POST("/pickup-assignments/:assignmentId/collect", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.update_status"), h.MarkCollected)
	rg.POST("/pickup-assignments/:assignmentId/deliver", rbacmiddleware.RequirePermission(rbacSvc, "dispatch.update_status"), h.MarkDelivered)
}
