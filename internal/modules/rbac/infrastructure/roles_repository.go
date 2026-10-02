package infrastructure

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	rbacdomain "dispatch/internal/modules/rbac/domain"
	platformdb "dispatch/internal/platform/db"
)

// ErrRoleInUse is returned when a delete is blocked because the role is still
// assigned to one or more users.
var ErrRoleInUse = errors.New("role is assigned to users")

// ErrSystemRole is returned when a mutation is blocked because the role is a
// protected, system-defined role.
var ErrSystemRole = errors.New("system roles cannot be modified this way")

// ErrRoleNotFound is returned when a role id does not exist.
var ErrRoleNotFound = errors.New("role not found")

func (r *Repository) ListPermissions(ctx context.Context) ([]rbacdomain.Permission, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, module, COALESCE(description,'')
		FROM permissions
		ORDER BY module, code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]rbacdomain.Permission, 0)
	for rows.Next() {
		var p rbacdomain.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Module, &p.Description); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (r *Repository) ListRoles(ctx context.Context, p platformdb.Pagination) ([]rbacdomain.Role, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM roles`).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT r.id, r.code, r.name, COALESCE(r.description,''), r.is_system,
		       (SELECT count(*) FROM role_permissions rp WHERE rp.role_id = r.id),
		       (SELECT count(DISTINCT ur.user_id) FROM user_roles ur WHERE ur.role_id = r.id AND ur.active),
		       r.created_at, r.updated_at
		FROM roles r
		ORDER BY r.is_system DESC, r.name
		LIMIT $1 OFFSET $2`, p.PageSize, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := make([]rbacdomain.Role, 0)
	for rows.Next() {
		var role rbacdomain.Role
		if err := rows.Scan(&role.ID, &role.Code, &role.Name, &role.Description, &role.IsSystem,
			&role.PermissionCount, &role.MemberCount, &role.CreatedAt, &role.UpdatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, role)
	}
	return out, total, rows.Err()
}

func (r *Repository) GetRole(ctx context.Context, id string) (rbacdomain.RoleDetail, error) {
	var d rbacdomain.RoleDetail
	err := r.db.QueryRow(ctx, `
		SELECT r.id, r.code, r.name, COALESCE(r.description,''), r.is_system,
		       (SELECT count(*) FROM role_permissions rp WHERE rp.role_id = r.id),
		       (SELECT count(DISTINCT ur.user_id) FROM user_roles ur WHERE ur.role_id = r.id AND ur.active),
		       r.created_at, r.updated_at
		FROM roles r WHERE r.id = $1`, id).Scan(
		&d.ID, &d.Code, &d.Name, &d.Description, &d.IsSystem,
		&d.PermissionCount, &d.MemberCount, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return rbacdomain.RoleDetail{}, ErrRoleNotFound
		}
		return rbacdomain.RoleDetail{}, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.code, p.name, p.module, COALESCE(p.description,'')
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1
		ORDER BY p.module, p.code`, id)
	if err != nil {
		return rbacdomain.RoleDetail{}, err
	}
	defer rows.Close()

	d.Permissions = make([]rbacdomain.Permission, 0)
	for rows.Next() {
		var p rbacdomain.Permission
		if err := rows.Scan(&p.ID, &p.Code, &p.Name, &p.Module, &p.Description); err != nil {
			return rbacdomain.RoleDetail{}, err
		}
		d.Permissions = append(d.Permissions, p)
	}
	return d, rows.Err()
}

func (r *Repository) CreateRole(ctx context.Context, code, name, description string) (rbacdomain.Role, error) {
	var role rbacdomain.Role
	err := r.db.QueryRow(ctx, `
		INSERT INTO roles (code, name, description, is_system)
		VALUES ($1, $2, NULLIF($3,''), FALSE)
		RETURNING id, code, name, COALESCE(description,''), is_system, 0, 0, created_at, updated_at`,
		code, name, description).Scan(
		&role.ID, &role.Code, &role.Name, &role.Description, &role.IsSystem,
		&role.PermissionCount, &role.MemberCount, &role.CreatedAt, &role.UpdatedAt)
	return role, err
}

func (r *Repository) UpdateRole(ctx context.Context, id, name, description string) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE roles
		SET name = $2, description = NULLIF($3,''), updated_at = now()
		WHERE id = $1`, id, name, description)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

func (r *Repository) DeleteRole(ctx context.Context, id string) error {
	var memberCount int
	if err := r.db.QueryRow(ctx,
		`SELECT count(*) FROM user_roles WHERE role_id = $1 AND active`, id).Scan(&memberCount); err != nil {
		return err
	}
	if memberCount > 0 {
		return ErrRoleInUse
	}
	ct, err := r.db.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrRoleNotFound
	}
	return nil
}

// IsSystemRole reports whether a role id belongs to a protected system role.
func (r *Repository) IsSystemRole(ctx context.Context, id string) (bool, error) {
	var isSystem bool
	err := r.db.QueryRow(ctx, `SELECT is_system FROM roles WHERE id = $1`, id).Scan(&isSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrRoleNotFound
	}
	return isSystem, err
}

// SetRolePermissions replaces a role's permission set with the given permission
// ids, atomically.
func (r *Repository) SetRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM role_permissions WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	if len(permissionIDs) > 0 {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_id)
			SELECT $1, pid FROM unnest($2::uuid[]) AS pid
			ON CONFLICT (role_id, permission_id) DO NOTHING`, roleID, permissionIDs); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// RolePermissionCodes returns the permission codes attached to a role.
func (r *Repository) RolePermissionCodes(ctx context.Context, roleID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.code
		FROM role_permissions rp
		JOIN permissions p ON p.id = rp.permission_id
		WHERE rp.role_id = $1`, roleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}

// EffectivePermissionCodes returns the distinct permission codes a user holds
// across all their active role assignments.
func (r *Repository) EffectivePermissionCodes(ctx context.Context, userID string) ([]string, error) {
	rows, err := r.db.Query(ctx, `
		SELECT DISTINCT p.code
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		JOIN permissions p ON p.id = rp.permission_id
		WHERE ur.user_id = $1 AND ur.active`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return nil, err
		}
		out = append(out, code)
	}
	return out, rows.Err()
}
