package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"dispatch/internal/platform/auth"
	"dispatch/internal/platform/db"
	"dispatch/internal/platform/events"

	"dispatch/internal/modules/users/application/dto"
	"dispatch/internal/modules/users/domain"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

var ErrUserNotFound = errors.New("user not found")

// DefaultTemporaryPassword is assigned to a new user when the creator does not
// supply a password. The user is expected to change it from their profile.
const DefaultTemporaryPassword = "123456789"

// ErrForbidden is returned when a caller attempts an admin password reset
// without the required privilege.
var ErrForbidden = errors.New("forbidden")

type Repository interface {
	Create(ctx context.Context, user domain.User, passwordHash string, profile dto.CreateUserRequest) error
	List(ctx context.Context, params dto.ListUsersParams) ([]domain.User, int64, error)
	GetByID(ctx context.Context, id string) (domain.User, error)
	Update(ctx context.Context, id string, req dto.UpdateUserRequest) (domain.User, error)
	Delete(ctx context.Context, id string) error
	SetActive(ctx context.Context, id string, active bool) (domain.User, error)

	GetPasswordHash(ctx context.Context, userID string) (string, error)
	ChangePassword(ctx context.Context, userID, newHash string) error

	AssignRole(ctx context.Context, userID string, req dto.AssignRoleRequest) error
	RemoveRole(ctx context.Context, userID string, roleID string) error
	ListRoles(ctx context.Context, userID string) ([]dto.UserRoleResponse, error)

	AssignUser(ctx context.Context, userID string, req dto.AssignUserRequest) error
	UpdateAssignment(ctx context.Context, assignmentID string, req dto.AssignUserRequest) error
	ListAssignments(ctx context.Context, userID string) ([]dto.UserAssignmentResponse, error)

	AssignCapability(ctx context.Context, userID string, req dto.AssignCapabilityRequest) error
	UpdateCapability(ctx context.Context, capabilityRecordID string, req dto.AssignCapabilityRequest) error
	ListCapabilities(ctx context.Context, userID string) ([]dto.UserCapabilityResponse, error)

	GetProfile(ctx context.Context, userID string) (map[string]any, error)
	UpdateProfile(ctx context.Context, userID string, req dto.UpdateUserProfileRequest) error
}

type Service struct {
	repo  Repository
	bus   events.Publisher
	log   *zap.Logger
	topic string
}

func NewService(repo Repository, bus events.Publisher, log *zap.Logger, topic string) *Service {
	return &Service{repo: repo, bus: bus, log: log, topic: topic}
}

func (s *Service) Create(ctx context.Context, req dto.CreateUserRequest) (domain.User, error) {
	// Fall back to a shared temporary password when none is supplied, so admins
	// can create accounts quickly; the user changes it from their profile.
	password := strings.TrimSpace(req.Password)
	if password == "" {
		password = DefaultTemporaryPassword
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return domain.User{}, err
	}

	u := domain.User{
		ID:        uuid.NewString(),
		Username:  req.Username,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
		Email:     req.Email,
		Status:    "ACTIVE",
		CreatedAt: time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, u, hash, req); err != nil {
		return domain.User{}, err
	}

	_ = s.bus.Publish(ctx, s.topic, events.Event{
		ID:          uuid.NewString(),
		Topic:       s.topic,
		AggregateID: u.ID,
		Type:        "user.created",
		OccurredAt:  time.Now().UTC(),
		Payload: map[string]any{
			"user_id":  u.ID,
			"username": u.Username,
		},
	})

	return u, nil
}

func (s *Service) List(ctx context.Context, params dto.ListUsersParams) (db.PageResult[domain.User], error) {
	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return db.PageResult[domain.User]{}, err
	}
	return db.PageResult[domain.User]{
		Items: items,
		Meta:  db.NewPageMeta(params.Pagination, total),
	}, nil
}

func (s *Service) GetByID(ctx context.Context, id string) (domain.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Update(ctx context.Context, id string, req dto.UpdateUserRequest) (domain.User, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) Activate(ctx context.Context, id string) (domain.User, error) {
	return s.repo.SetActive(ctx, id, true)
}

func (s *Service) Deactivate(ctx context.Context, id string) (domain.User, error) {
	return s.repo.SetActive(ctx, id, false)
}

// ChangePassword updates a user's password. Two flows are supported:
//
//   - Self-service: the caller changes their own password (targetID == callerID)
//     and must supply the correct current password. reset_by_admin is ignored.
//   - Admin reset: the caller resets another user's password, or explicitly sets
//     reset_by_admin. This requires callerCanAdminReset (the users.update
//     privilege), checked by the handler; without it the call is forbidden.
//
// This split is what closes the privilege-escalation hole where any user could
// reset anyone's password by setting reset_by_admin.
func (s *Service) ChangePassword(ctx context.Context, targetID string, req dto.ChangePasswordRequest, callerID string, callerCanAdminReset bool) error {
	isSelf := targetID == callerID
	adminReset := req.ResetByAdmin || !isSelf

	if adminReset {
		if !callerCanAdminReset {
			return ErrForbidden
		}
	} else {
		hash, err := s.repo.GetPasswordHash(ctx, targetID)
		if err != nil {
			return err
		}
		if err := auth.CheckPassword(hash, req.CurrentPassword); err != nil {
			return errors.New("current password is incorrect")
		}
	}

	newHash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	return s.repo.ChangePassword(ctx, targetID, newHash)
}

func (s *Service) AssignRole(ctx context.Context, userID string, req dto.AssignRoleRequest) error {
	return s.repo.AssignRole(ctx, userID, req)
}

func (s *Service) RemoveRole(ctx context.Context, userID string, roleID string) error {
	return s.repo.RemoveRole(ctx, userID, roleID)
}

func (s *Service) AssignUser(ctx context.Context, userID string, req dto.AssignUserRequest) error {
	return s.repo.AssignUser(ctx, userID, req)
}

func (s *Service) UpdateAssignment(ctx context.Context, assignmentID string, req dto.AssignUserRequest) error {
	return s.repo.UpdateAssignment(ctx, assignmentID, req)
}

func (s *Service) AssignCapability(ctx context.Context, userID string, req dto.AssignCapabilityRequest) error {
	return s.repo.AssignCapability(ctx, userID, req)
}

func (s *Service) UpdateCapability(ctx context.Context, capabilityRecordID string, req dto.AssignCapabilityRequest) error {
	return s.repo.UpdateCapability(ctx, capabilityRecordID, req)
}

func (s *Service) UpdateProfile(ctx context.Context, userID string, req dto.UpdateUserProfileRequest) error {
	return s.repo.UpdateProfile(ctx, userID, req)
}

func (s *Service) GetDetails(ctx context.Context, userID string) (dto.UserDetailsResponse, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return dto.UserDetailsResponse{}, err
	}

	profile, err := s.repo.GetProfile(ctx, userID)
	if err != nil {
		return dto.UserDetailsResponse{}, err
	}

	roles, err := s.repo.ListRoles(ctx, userID)
	if err != nil {
		return dto.UserDetailsResponse{}, err
	}

	assignments, err := s.repo.ListAssignments(ctx, userID)
	if err != nil {
		return dto.UserDetailsResponse{}, err
	}

	capabilities, err := s.repo.ListCapabilities(ctx, userID)
	if err != nil {
		return dto.UserDetailsResponse{}, err
	}

	return dto.UserDetailsResponse{
		User:         user,
		Profile:      profile,
		Roles:        roles,
		Assignments:  assignments,
		Capabilities: capabilities,
	}, nil
}
