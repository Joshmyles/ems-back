package http

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the self-service profile endpoints. The parent group
// already enforces authentication, so every handler acts on the caller.
func RegisterRoutes(rg *gin.RouterGroup, h *Handler) {
	rg.GET("", h.Me)
	rg.PATCH("", h.UpdateMe)
	rg.POST("/change-password", h.ChangeMyPassword)
}
