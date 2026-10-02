package types

import "context"

// Audit action codes. Keep these stable — they are written to the audit trail
// and queried by the audit viewer. Grouped as "<entity>.<verb>".
const (
	AuditAuthLogin       = "auth.login"
	AuditAuthLoginFailed = "auth.login_failed"
	AuditAuthLogout      = "auth.logout"
	AuditAuthLogoutAll   = "auth.logout_all"

	AuditUserCreate         = "user.create"
	AuditUserUpdate         = "user.update"
	AuditUserDelete         = "user.delete"
	AuditUserPasswordChange = "user.password_change"
	AuditUserPasswordReset  = "user.password_reset"
	AuditUserProfileUpdate  = "user.profile_update"
	AuditUserAssignScope    = "user.assign_scope"

	AuditRoleCreate      = "role.create"
	AuditRoleUpdate      = "role.update"
	AuditRoleDelete      = "role.delete"
	AuditRolePermsSet    = "role.permissions_set"
	AuditUserRoleGrant   = "user_role.grant"
	AuditUserRoleRevoke  = "user_role.revoke"
	AuditUserRoleDenied  = "user_role.grant_denied"

	AuditAccessDenied = "access.denied"

	AuditReportExport = "report.export"
)

// Audit status values.
const (
	AuditStatusSuccess = "SUCCESS"
	AuditStatusFailure = "FAILURE"
	AuditStatusDenied  = "DENIED"
)

// AuditEntry is a single audit-trail record. Modules build this and hand it to
// the AuditRecorder; they never touch the audit storage directly. All fields
// are optional except Action — recording is best-effort and must never break
// the request it describes.
type AuditEntry struct {
	ActorUserID   string         // UUID of the acting user ("" when unknown, e.g. failed login)
	ActorUsername string         // denormalised for display
	ActorRoles    []string       // role codes the actor carried
	Action        string         // one of the Audit* constants
	EntityType    string         // e.g. "user", "role", "user_role"
	EntityID      string         // UUID of the affected entity ("" when none)
	Status        string         // SUCCESS / FAILURE / DENIED (defaults to SUCCESS)
	Description   string         // short human-readable summary
	Before        any            // prior state for updates (JSON-encoded)
	After         any            // new state for creates/updates (JSON-encoded)
	IPAddress     string         // client IP ("" when unknown)
	UserAgent     string         // client user-agent
	RequestID     string         // correlation id if available
	Metadata      map[string]any // extra structured context
}

// AuditRecorder persists audit entries. Implementations must be safe for
// concurrent use and must swallow their own errors (logging them) so that a
// failed audit write never propagates into the caller's request path.
type AuditRecorder interface {
	Record(ctx context.Context, entry AuditEntry)
}

// NoopAuditRecorder discards every entry. Used as a safe default when auditing
// has not been wired up, so call sites never need a nil check.
type NoopAuditRecorder struct{}

func (NoopAuditRecorder) Record(context.Context, AuditEntry) {}
