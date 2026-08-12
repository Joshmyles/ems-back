package blood

import (
	bloodapp "dispatch/internal/modules/blood/application"
	"dispatch/internal/modules/blood/infrastructure"
	"dispatch/internal/modules/blood/infrastructure/http"
	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/shared/types"
)

func Register(deps types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := infrastructure.NewRepository(deps.DB)
	service := bloodapp.NewService(repo, deps.Bus, deps.Logger)
	handler := http.NewHandler(service, rbacSvc)
	group := deps.Router.Group("/blood")
	http.RegisterRoutes(group, handler, rbacSvc)
}
