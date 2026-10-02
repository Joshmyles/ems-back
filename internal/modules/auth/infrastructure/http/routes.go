package http

import "github.com/gin-gonic/gin"

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, authMiddleware gin.HandlerFunc) {
	rg.POST("/login", h.Login)
	rg.POST("/refresh", h.Refresh)
	secured := rg.Group("")
	secured.Use(authMiddleware)
	secured.POST("/logout", h.Logout)
	secured.GET("/sessions", h.Sessions)
	secured.DELETE("/sessions/:id", h.RevokeSession)
}
