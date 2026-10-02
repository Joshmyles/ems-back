package bootstrap

import (
	analyticsmod "dispatch/internal/modules/analytics"
	auditmod "dispatch/internal/modules/audit"
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
	profilemod "dispatch/internal/modules/profile"
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
	// Build the audit recorder first and share it with every module via
	// deps.Audit, so security-relevant actions (incl. login) are recorded.
	auditRecorder := auditmod.NewRecorder(deps)
	deps.Audit = auditRecorder

	authmod.Register(deps)

	refmod.Register(deps)
	rbacSvc := rbacmod.BuildService(deps)

	secured := deps.Router.Group("")
	secured.Use(
		authmiddleware.AuthMiddleware(deps.Config.JWT.Secret),
		rbacmiddleware.ScopeContextMiddleware(),
		rbacmiddleware.AuditDenials(auditRecorder),
	)

	// Copy every dependency (incl. PushSender, Audit) — only the router changes.
	securedDeps := deps
	securedDeps.Router = secured

	auditmod.RegisterRoutes(securedDeps, auditRecorder, rbacSvc)
	rbacmod.RegisterRoutes(securedDeps, rbacSvc)
	profilemod.Register(securedDeps, rbacSvc)
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
	analyticsmod.Register(securedDeps, rbacSvc)
}
