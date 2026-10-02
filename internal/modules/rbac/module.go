package rbac

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/modules/rbac/infrastructure"
	"dispatch/internal/modules/rbac/infrastructure/http"
	"dispatch/internal/shared/types"
)

func BuildService(deps types.ModuleDeps) *rbacapp.Service {
	repo := infrastructure.NewRepository(deps.DB)
	return rbacapp.NewService(repo, deps.Redis, deps.Logger)
}

func RegisterRoutes(deps types.ModuleDeps, service *rbacapp.Service) {
	h := http.NewHandler(service, deps.Audit)
	group := deps.Router.Group("/rbac")
	http.RegisterRoutes(group, h, service)
}
