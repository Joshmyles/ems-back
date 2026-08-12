package notifications

import (
	notifapp "dispatch/internal/modules/notifications/application"
	"dispatch/internal/modules/notifications/infrastructure"
	"dispatch/internal/modules/notifications/infrastructure/http"
	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/shared/types"
)

func Register(deps types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := infrastructure.NewRepository(deps.DB)
	service := notifapp.NewService(repo, deps.Bus, deps.Logger, deps.PushSender)
	handler := http.NewHandler(service)
	group := deps.Router.Group("/notifications")
	http.RegisterRoutes(group, handler, rbacSvc)
}
