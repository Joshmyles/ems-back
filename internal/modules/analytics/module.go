package analytics

import (
	analyticsapp "dispatch/internal/modules/analytics/application"
	"dispatch/internal/modules/analytics/infrastructure"
	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/shared/types"
)

func Register(deps types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := infrastructure.NewRepository(deps.DB)
	service := analyticsapp.NewService(repo)
	handler := infrastructure.NewHandler(service, deps.Logger, deps.Audit)

	group := deps.Router.Group("/analytics")
	infrastructure.RegisterRoutes(group, handler, rbacSvc)
}
