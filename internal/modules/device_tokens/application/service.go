package application

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	devicedomain "dispatch/internal/modules/device_tokens/domain"
	platformdb "dispatch/internal/platform/db"
)

var ErrDeviceTokenNotFound = errors.New("device token not found")

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Register(ctx context.Context, req RegisterDeviceTokenRequest) (devicedomain.DeviceToken, error) {
	in := devicedomain.DeviceToken{
		ID:        uuid.NewString(),
		UserID:    req.UserID,
		DeviceID:  strings.TrimSpace(req.DeviceID),
		Platform:  normalizePlatform(req.Platform),
		PushToken: strings.TrimSpace(req.PushToken),
		IsActive:  true,
	}
	return s.repo.Register(ctx, in)
}

// normalizePlatform maps the client value onto the user_device_tokens CHECK
// constraint (ANDROID, IOS, WEB). Older mobile builds send no platform at
// login; defaulting to ANDROID keeps their FCM token instead of rejecting it.
func normalizePlatform(p string) string {
	switch v := strings.ToUpper(strings.TrimSpace(p)); v {
	case "ANDROID", "IOS", "WEB":
		return v
	default:
		return "ANDROID"
	}
}

func (s *Service) Update(ctx context.Context, id string, req UpdateDeviceTokenRequest) (devicedomain.DeviceToken, error) {
	if req.Platform != nil {
		v := normalizePlatform(*req.Platform)
		req.Platform = &v
	}
	if req.DeviceID != nil {
		v := strings.TrimSpace(*req.DeviceID)
		req.DeviceID = &v
	}
	if req.PushToken != nil {
		v := strings.TrimSpace(*req.PushToken)
		req.PushToken = &v
	}
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Deactivate(ctx context.Context, id string) error {
	return s.repo.Deactivate(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id string) (devicedomain.DeviceToken, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, params ListDeviceTokensParams) (platformdb.PageResult[devicedomain.DeviceToken], error) {
	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return platformdb.PageResult[devicedomain.DeviceToken]{}, err
	}
	return platformdb.PageResult[devicedomain.DeviceToken]{
		Items: items,
		Meta:  platformdb.NewPageMeta(params.Pagination, total),
	}, nil
}

func (s *Service) GetPushTokensByUserID(ctx context.Context, userID string) ([]string, error) {
	return s.repo.GetPushTokensByUserID(ctx, userID)
}

func nowPtr() *time.Time {
	t := time.Now().UTC()
	return &t
}
