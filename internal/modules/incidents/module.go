package incidents

import (
	incapp "dispatch/internal/modules/incidents/application"
	"dispatch/internal/modules/incidents/infrastructure"
	"dispatch/internal/modules/incidents/infrastructure/http"
	notificationsapp "dispatch/internal/modules/notifications/application"
	notificationsinfra "dispatch/internal/modules/notifications/infrastructure"
	"dispatch/internal/shared/types"

	middleware "dispatch/internal/modules/auth/middleware"
	rbacapp "dispatch/internal/modules/rbac/application"
)

func Register(deps types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := infrastructure.NewRepository(deps.DB)
	notificationRepo := notificationsinfra.NewRepository(deps.DB)
	notificationService := notificationsapp.NewService(notificationRepo, deps.Bus, deps.Logger)
	service := incapp.NewService(repo, deps.Bus, deps.Logger, notificationService)
	handler := http.NewHandler(service)
	group := deps.Router.Group("/incidents")
	http.RegisterRoutes(group, handler, rbacSvc, middleware.AuthMiddleware(deps.Config.JWT.Secret))
}
