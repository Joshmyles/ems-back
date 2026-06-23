package application

import (
	"context"
	"strings"
	"time"

	"dispatch/internal/modules/notifications/domain"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/events"
	"dispatch/internal/shared/constants"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type Service struct {
	repo Repository
	log  *zap.Logger
	bus  events.Publisher
}

func NewService(repo Repository, bus events.Publisher, log *zap.Logger) *Service {
	return &Service{repo: repo, bus: bus, log: log}
}

func (s *Service) ListMy(ctx context.Context, userID string, p platformdb.Pagination) (platformdb.PageResult[domain.Notification], error) {
	items, total, err := s.repo.ListNotifications(ctx, userID, p)
	if err != nil {
		return platformdb.PageResult[domain.Notification]{}, err
	}
	return platformdb.PageResult[domain.Notification]{Items: items, Meta: platformdb.NewPageMeta(p, total)}, nil
}

func (s *Service) Get(ctx context.Context, id string) (domain.Notification, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) Create(ctx context.Context,
	typ, channel string,
	recipientUserID, recipientPhone, recipientEmail, title, linkedEntityType *string,
	body string,
	linkedEntityID *string,

) (domain.Notification, error) {
	now := time.Now().UTC()
	n := domain.Notification{
		ID:               uuid.NewString(),
		Type:             typ,
		RecipientUserID:  recipientUserID,
		RecipientPhone:   recipientPhone,
		RecipientEmail:   recipientEmail,
		Title:            title,
		Body:             body,
		Channel:          channel,
		LinkedEntityType: linkedEntityType,
		LinkedEntityID:   linkedEntityID,
		Status:           "PENDING",
		Attempts:         0,
		CreatedAt:        now,
	}
	created, err := s.repo.Create(ctx, n)
	if err != nil {
		return domain.Notification{}, err
	}

	_ = s.bus.Publish(ctx, constants.TopicNotificationSendRequested, events.Event{
		ID:          uuid.NewString(),
		Topic:       constants.TopicNotificationSendRequested,
		AggregateID: created.ID,
		Type:        constants.TopicNotificationSendRequested,
		OccurredAt:  time.Now().UTC(),
		Payload: map[string]any{
			"notification_id":    created.ID,
			"type":               created.Type,
			"channel":            created.Channel,
			"recipient_user_id":  created.RecipientUserID,
			"recipient_phone":    created.RecipientPhone,
			"recipient_email":    created.RecipientEmail,
			"title":              created.Title,
			"body":               created.Body,
			"linked_entity_type": created.LinkedEntityType,
			"linked_entity_id":   created.LinkedEntityID,
		},
	})

	return created, nil
}

func (s *Service) UpdateStatus(ctx context.Context, id string, status string) error {
	return s.repo.UpdateStatus(ctx, id, strings.ToUpper(strings.TrimSpace(status)))
}

func (s *Service) UpdateStatusForUser(ctx context.Context, id string, userID string, status string) error {
	return s.repo.UpdateStatusForUser(ctx, id, userID, strings.ToUpper(strings.TrimSpace(status)))
}
