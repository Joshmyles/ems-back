package application

import (
	"context"

	rbacdomain "dispatch/internal/modules/rbac/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository interface {
	ListPermissionGrants(ctx context.Context, userID string) ([]rbacdomain.PermissionGrant, error)

	// Permission catalogue & role management.
	ListPermissions(ctx context.Context) ([]rbacdomain.Permission, error)
	ListRoles(ctx context.Context, p platformdb.Pagination) ([]rbacdomain.Role, int64, error)
	GetRole(ctx context.Context, id string) (rbacdomain.RoleDetail, error)
	CreateRole(ctx context.Context, code, name, description string) (rbacdomain.Role, error)
	UpdateRole(ctx context.Context, id, name, description string) error
	DeleteRole(ctx context.Context, id string) error
	IsSystemRole(ctx context.Context, id string) (bool, error)
	SetRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error
	RolePermissionCodes(ctx context.Context, roleID string) ([]string, error)
	EffectivePermissionCodes(ctx context.Context, userID string) ([]string, error)
}
