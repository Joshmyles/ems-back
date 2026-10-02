package audit

import (
	auditapp "dispatch/internal/modules/audit/application"
	auditinfra "dispatch/internal/modules/audit/infrastructure"
	audithttp "dispatch/internal/modules/audit/infrastructure/http"
	rbacapp "dispatch/internal/modules/rbac/application"
	"dispatch/internal/shared/types"
)

// NewRecorder builds the audit recorder. Bootstrap calls this before module
// registration so every module shares one recorder via ModuleDeps.Audit.
func NewRecorder(deps types.ModuleDeps) *auditapp.Service {
	repo := auditinfra.NewRepository(deps.DB)
	return auditapp.NewService(repo, deps.Logger)
}

// RegisterRoutes mounts the read-only audit API. The same *auditapp.Service
// that records entries also answers queries, so pass the recorder back in.
func RegisterRoutes(deps types.ModuleDeps, recorder *auditapp.Service, rbacSvc *rbacapp.Service) {
	h := audithttp.NewHandler(recorder)
	group := deps.Router.Group("/audit")
	audithttp.RegisterRoutes(group, h, rbacSvc)
}
