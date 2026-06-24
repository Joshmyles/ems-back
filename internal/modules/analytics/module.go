package analytics

import (
	analyticsapp "dispatch/internal/modules/analytics/application"
	"dispatch/internal/modules/analytics/infrastructure"
	"dispatch/internal/shared/types"
)

func Register(deps types.ModuleDeps) {
	repo := infrastructure.NewRepository(deps.DB)
	service := analyticsapp.NewService(repo)
	handler := infrastructure.NewHandler(service)

	group := deps.Router.Group("/analytics")
	infrastructure.RegisterRoutes(group, handler)
}
