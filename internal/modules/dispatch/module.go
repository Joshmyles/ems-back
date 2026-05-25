package dispatch

import (
	dispatchapp "dispatch/internal/modules/dispatch/application"
	"dispatch/internal/modules/dispatch/infrastructure"
	"dispatch/internal/modules/dispatch/infrastructure/http"
	notificationsapp "dispatch/internal/modules/notifications/application"
	notificationsinfra "dispatch/internal/modules/notifications/infrastructure"
	"dispatch/internal/shared/types"
)

func Register(deps types.ModuleDeps) {
	repo := infrastructure.NewRepository(deps.DB)
	notificationRepo := notificationsinfra.NewRepository(deps.DB)
	notificationService := notificationsapp.NewService(notificationRepo, deps.Bus, deps.Logger)
	service := dispatchapp.NewService(repo, deps.Bus, notificationService)
	h := http.NewHandler(service)
	group := deps.Router.Group("/dispatch")
	http.RegisterRoutes(group, h)
}
