package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/shared/types"
)

func ScopeContextMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// AuditDenials records every authorization denial (HTTP 403) for an
// authenticated user, giving the audit trail a complete record of attempted
// access that was refused. It runs after the handler chain and reads the final
// status. Handlers that already recorded a richer, domain-specific denial set
// "audit_denial_recorded" to avoid a duplicate generic entry.
func AuditDenials(rec types.AuditRecorder) gin.HandlerFunc {
	if rec == nil {
		rec = types.NoopAuditRecorder{}
	}
	return func(c *gin.Context) {
		c.Next()

		if c.Writer.Status() != http.StatusForbidden {
			return
		}
		if c.GetBool("audit_denial_recorded") {
			return
		}
		userID := c.GetString("user_id")
		if userID == "" {
			return // unauthenticated noise; not an access-control decision about a known actor
		}

		var roles []string
		if v, ok := c.Get("roles"); ok {
			roles, _ = v.([]string)
		}
		rec.Record(c.Request.Context(), types.AuditEntry{
			ActorUserID:   userID,
			ActorUsername: c.GetString("username"),
			ActorRoles:    roles,
			Action:        types.AuditAccessDenied,
			EntityType:    "endpoint",
			Status:        types.AuditStatusDenied,
			Description:   "Access denied: " + c.Request.Method + " " + routePath(c),
			IPAddress:     c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
			Metadata:      map[string]any{"method": c.Request.Method, "path": routePath(c)},
		})
	}
}

func routePath(c *gin.Context) string {
	if p := c.FullPath(); p != "" {
		return p
	}
	return c.Request.URL.Path
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

// RequireSelfOrPermission allows the request through when the authenticated
// user is acting on their own record (the ":id" path param equals their user
// id); otherwise it falls back to the normal permission check. Used for
// endpoints that users may call on themselves (change own password, update own
// profile) but which require a privilege to call against another user.
func RequireSelfOrPermission(rbacSvc *rbacapp.Service, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID != "" && c.Param("id") == userID {
			c.Next()
			return
		}
		RequirePermission(rbacSvc, permission)(c)
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
