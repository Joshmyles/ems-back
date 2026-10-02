package application

import (
	"context"
	"errors"
	"strings"

	rbacdomain "dispatch/internal/modules/rbac/domain"
	platformdb "dispatch/internal/platform/db"
)

// ErrPrivilegeEscalation is returned when an actor tries to grant permissions
// they do not themselves hold.
var ErrPrivilegeEscalation = errors.New("cannot grant permissions you do not hold")

func (s *Service) ListPermissions(ctx context.Context) ([]rbacdomain.Permission, error) {
	if s == nil || s.repo == nil {
		return nil, ErrRBACNotInitialized
	}
	return s.repo.ListPermissions(ctx)
}

func (s *Service) ListRoles(ctx context.Context, p platformdb.Pagination) (platformdb.PageResult[rbacdomain.Role], error) {
	if s == nil || s.repo == nil {
		return platformdb.PageResult[rbacdomain.Role]{}, ErrRBACNotInitialized
	}
	items, total, err := s.repo.ListRoles(ctx, p)
	if err != nil {
		return platformdb.PageResult[rbacdomain.Role]{}, err
	}
	return platformdb.PageResult[rbacdomain.Role]{Items: items, Meta: platformdb.NewPageMeta(p, total)}, nil
}

func (s *Service) GetRole(ctx context.Context, id string) (rbacdomain.RoleDetail, error) {
	if s == nil || s.repo == nil {
		return rbacdomain.RoleDetail{}, ErrRBACNotInitialized
	}
	return s.repo.GetRole(ctx, id)
}

func (s *Service) CreateRole(ctx context.Context, code, name, description string) (rbacdomain.Role, error) {
	if s == nil || s.repo == nil {
		return rbacdomain.Role{}, ErrRBACNotInitialized
	}
	code = strings.ToUpper(strings.TrimSpace(strings.ReplaceAll(code, " ", "_")))
	return s.repo.CreateRole(ctx, code, strings.TrimSpace(name), strings.TrimSpace(description))
}

func (s *Service) UpdateRole(ctx context.Context, id, name, description string) error {
	if s == nil || s.repo == nil {
		return ErrRBACNotInitialized
	}
	return s.repo.UpdateRole(ctx, id, strings.TrimSpace(name), strings.TrimSpace(description))
}

func (s *Service) DeleteRole(ctx context.Context, id string) error {
	if s == nil || s.repo == nil {
		return ErrRBACNotInitialized
	}
	return s.repo.DeleteRole(ctx, id)
}

func (s *Service) IsSystemRole(ctx context.Context, id string) (bool, error) {
	if s == nil || s.repo == nil {
		return false, ErrRBACNotInitialized
	}
	return s.repo.IsSystemRole(ctx, id)
}

func (s *Service) SetRolePermissions(ctx context.Context, roleID string, permissionIDs []string) error {
	if s == nil || s.repo == nil {
		return ErrRBACNotInitialized
	}
	return s.repo.SetRolePermissions(ctx, roleID, permissionIDs)
}

func (s *Service) RolePermissionCodes(ctx context.Context, roleID string) ([]string, error) {
	if s == nil || s.repo == nil {
		return nil, ErrRBACNotInitialized
	}
	return s.repo.RolePermissionCodes(ctx, roleID)
}

// EnsureCanGrant enforces the privilege-escalation guard: an actor may only
// grant a set of permissions that is a subset of the permissions they already
// hold. A SUPER_ADMIN (by role or by holding a wildcard permission) bypasses
// the check. Returns ErrPrivilegeEscalation (wrapping the missing codes) when
// the actor is under-privileged.
func (s *Service) EnsureCanGrant(ctx context.Context, actorID string, actorRoles, requiredCodes []string) error {
	if s == nil || s.repo == nil {
		return ErrRBACNotInitialized
	}
	for _, role := range actorRoles {
		if strings.EqualFold(strings.TrimSpace(role), "SUPER_ADMIN") {
			return nil
		}
	}

	held, err := s.repo.EffectivePermissionCodes(ctx, actorID)
	if err != nil {
		return err
	}
	heldNorm := make([]string, 0, len(held))
	for _, h := range held {
		h = normalize(h)
		if h == "*" || h == "*.*" {
			return nil // actor holds everything
		}
		heldNorm = append(heldNorm, h)
	}

	var missing []string
	for _, req := range requiredCodes {
		req = normalize(req)
		covered := false
		for _, h := range heldNorm {
			if permissionMatches(h, req) {
				covered = true
				break
			}
		}
		if !covered {
			missing = append(missing, req)
		}
	}
	if len(missing) > 0 {
		return &EscalationError{Missing: missing}
	}
	return nil
}

// EscalationError carries the specific permission codes an actor lacked.
type EscalationError struct {
	Missing []string
}

func (e *EscalationError) Error() string {
	return ErrPrivilegeEscalation.Error() + ": " + strings.Join(e.Missing, ", ")
}

func (e *EscalationError) Unwrap() error { return ErrPrivilegeEscalation }
