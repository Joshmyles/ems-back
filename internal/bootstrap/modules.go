package bootstrap

import (
	authmod "dispatch/internal/modules/auth"
	availabilitymod "dispatch/internal/modules/availability"
	bloodmod "dispatch/internal/modules/blood"
	dashboard "dispatch/internal/modules/dashboard"
	devicetokens "dispatch/internal/modules/device_tokens"
	dispatchmod "dispatch/internal/modules/dispatch"
	facilitiesmod "dispatch/internal/modules/facilities"
	fleetmod "dispatch/internal/modules/fleet"
	fuelmod "dispatch/internal/modules/fuel"
	incidentmod "dispatch/internal/modules/incidents"
	notifmod "dispatch/internal/modules/notifications"
	rbacmod "dispatch/internal/modules/rbac"
	refmod "dispatch/internal/modules/reference"
	respmod "dispatch/internal/modules/responders"
	tripsmod "dispatch/internal/modules/trips"
	usermod "dispatch/internal/modules/users"
	"dispatch/internal/shared/types"

	authmiddleware "dispatch/internal/modules/auth/middleware"
	rbacmiddleware "dispatch/internal/modules/rbac/middleware"
)

func RegisterModules(deps types.ModuleDeps) {
	authmod.Register(deps)

	refmod.Register(deps)
	rbacSvc := rbacmod.BuildService(deps)

	secured := deps.Router.Group("")
	secured.Use(authmiddleware.AuthMiddleware(deps.Config.JWT.Secret), rbacmiddleware.ScopeContextMiddleware())

	securedDeps := types.ModuleDeps{
		Router: secured,
		DB:     deps.DB,
		Redis:  deps.Redis,
		Logger: deps.Logger,
		Bus:    deps.Bus,
		Config: deps.Config,
	}

	rbacmod.RegisterRoutes(securedDeps, rbacSvc)
	usermod.Register(securedDeps, rbacSvc)
	facilitiesmod.Register(securedDeps, rbacSvc)
	fleetmod.Register(securedDeps, rbacSvc)
	respmod.Register(securedDeps, rbacSvc)
	// incidents is registered on the unsecured router so the public can report
	// incidents (POST /incidents). Read/update routes re-apply AuthMiddleware
	// inside the module's RegisterRoutes.
	incidentmod.Register(deps, rbacSvc)
	bloodmod.Register(securedDeps, rbacSvc)
	tripsmod.Register(securedDeps, rbacSvc)
	notifmod.Register(securedDeps, rbacSvc)
	fuelmod.Register(securedDeps, deps, rbacSvc)
	availabilitymod.Register(securedDeps)
	dispatchmod.Register(securedDeps)
	devicetokens.Register(securedDeps)
	dashboard.Register(securedDeps)
}
