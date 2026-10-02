package fuel

import (
	fuelapp "dispatch/internal/modules/fuel/application"
	"dispatch/internal/modules/fuel/infrastructure"
	"dispatch/internal/modules/fuel/infrastructure/http"
	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/shared/types"
)

// Register wires the fuel module. The authenticated CRUD routes are mounted on
// secured, while the QR-scan verification routes are mounted on public (a
// router group with no auth middleware) so fuel station attendants can use
// them without an account.
func Register(secured types.ModuleDeps, public types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := infrastructure.NewRepository(secured.DB)
	service := fuelapp.NewService(repo, secured.Logger)
	handler := http.NewHandler(service, secured.Audit)

	group := secured.Router.Group("/fuel")
	http.RegisterRoutes(group, handler, rbacSvc)

	publicGroup := public.Router.Group("/public")
	http.RegisterPublicRoutes(publicGroup, handler)
}
