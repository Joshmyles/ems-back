package responders

import (
	rbacapp "dispatch/internal/modules/rbac/application"
	respapp "dispatch/internal/modules/responders/application"
	"dispatch/internal/modules/responders/infrastructure"
	"dispatch/internal/modules/responders/infrastructure/http"
	"dispatch/internal/shared/types"
)

func Register(deps types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := infrastructure.NewRepository(deps.DB)
	service := respapp.NewService(repo, deps.Logger)
	handler := http.NewHandler(service)
	group := deps.Router.Group("/responders")
	http.RegisterRoutes(group, handler, rbacSvc)
}
