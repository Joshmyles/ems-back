package users

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	userapp "dispatch/internal/modules/users/application"
	infra "dispatch/internal/modules/users/infrastructure"
	"dispatch/internal/modules/users/infrastructure/http"
	"dispatch/internal/shared/types"
)

func Register(deps types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := infra.NewRepository(deps.DB)
	service := userapp.NewService(repo, deps.Bus, deps.Logger, deps.Config.Kafka.TopicUserCreated)
	handler := http.NewHandler(service, rbacSvc, deps.Audit)
	group := deps.Router.Group("/users")
	http.RegisterRoutes(group, handler, rbacSvc)
}
