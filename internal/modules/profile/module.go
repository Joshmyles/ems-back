package profile

import (
	profilehttp "dispatch/internal/modules/profile/infrastructure/http"
	rbacapp "dispatch/internal/modules/rbac/application"
	userapp "dispatch/internal/modules/users/application"
	userinfra "dispatch/internal/modules/users/infrastructure"
	"dispatch/internal/shared/types"
)

// Register mounts the self-service profile ("/me") routes. It builds its own
// users service over the shared DB — the service is a stateless wrapper over
// the repository, so this avoids threading the users module's instance through.
func Register(deps types.ModuleDeps, rbacSvc *rbacapp.Service) {
	repo := userinfra.NewRepository(deps.DB)
	userSvc := userapp.NewService(repo, deps.Bus, deps.Logger, deps.Config.Kafka.TopicUserCreated)

	h := profilehttp.NewHandler(userSvc, rbacSvc, deps.Audit)
	group := deps.Router.Group("/me")
	profilehttp.RegisterRoutes(group, h)
}
