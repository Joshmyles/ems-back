package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
)

func ScopeContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func RequirePermission(rbacSvc *rbacapp.Service, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if rbacSvc == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "rbac service is not initialized!",
			})
			return
		}

		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "unauthenticated",
			})
			return
		}

		scopeType := c.GetString("scope_type")
		var scopeID *string
		if v := c.GetString("scope_id"); v != "" {
			scopeID = &v
		}

		ok, err := rbacSvc.HasPermission(c.Request.Context(), userID, permission, scopeType, scopeID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": "failed to evaluate permission",
			})
			return
		}
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"message":    "forbidden",
				"permission": permission,
			})
			return
		}

		c.Next()
	}
}

func RequirePermissionOrRole(rbacSvc *rbacapp.Service, permission string, roleCodes ...string) gin.HandlerFunc {
	requiredRoles := make(map[string]struct{}, len(roleCodes))
	for _, roleCode := range roleCodes {
		roleCode = strings.ToUpper(strings.TrimSpace(roleCode))
		if roleCode != "" {
			requiredRoles[roleCode] = struct{}{}
		}
	}

	return func(c *gin.Context) {
		if userHasAnyRole(c, requiredRoles) {
			c.Next()
			return
		}
		RequirePermission(rbacSvc, permission)(c)
	}
}

func userHasAnyRole(c *gin.Context, roleCodes map[string]struct{}) bool {
	if len(roleCodes) == 0 {
		return false
	}
	rawRoles, _ := c.Get("roles")
	roles, _ := rawRoles.([]string)
	for _, role := range roles {
		if _, ok := roleCodes[strings.ToUpper(strings.TrimSpace(role))]; ok {
			return true
		}
	}
	return false
}
