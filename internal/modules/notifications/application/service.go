package application

import (
	"context"
	"strings"
	"time"

	"dispatch/internal/modules/notifications/domain"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/events"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Sender delivers a notification to its final channel. Nil disables delivery
// (rows stay PENDING) — e.g. when Firebase credentials are not configured.
type Sender interface {
	SendSMS(ctx context.Context, to string, body string) error
	SendEmail(ctx context.Context, to, subject, body string) error
	SendPush(ctx context.Context, userID string, title, body string) error
}

type Service struct {
	repo   Repository
	log    *zap.Logger
	bus    events.Publisher
	sender Sender
}

func NewService(repo Repository, bus events.Publisher, log *zap.Logger, sender Sender) *Service {
	return &Service{repo: repo, bus: bus, log: log, sender: sender}
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

	// Deliver directly, off the request path. No message broker involved:
	// the send happens in-process and the row is marked SENT/FAILED here.
	go s.deliver(created)

	return created, nil
}

// deliver sends a notification on its channel and records the outcome. Runs
// in its own goroutine with a fresh context so a finished HTTP request does
// not cancel the send.
func (s *Service) deliver(n domain.Notification) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// IN_APP notifications are "delivered" by existing in the database.
	if n.Channel == "IN_APP" {
		_ = s.repo.MarkSent(ctx, n.ID)
		return
	}

	if s.sender == nil {
		s.log.Debug("notification sender not configured; leaving PENDING",
			zap.String("notification_id", n.ID), zap.String("channel", n.Channel))
		return
	}

	_ = s.repo.IncrementAttempts(ctx, n.ID)

	title := ""
	if n.Title != nil {
		title = *n.Title
	}

	var err error
	switch n.Channel {
	case "PUSH":
		recipient := ""
		if n.RecipientUserID != nil {
			recipient = *n.RecipientUserID
		}
		err = s.sender.SendPush(ctx, recipient, title, n.Body)
	case "SMS":
		phone := ""
		if n.RecipientPhone != nil {
			phone = *n.RecipientPhone
		}
		err = s.sender.SendSMS(ctx, phone, n.Body)
	case "EMAIL":
		email := ""
		if n.RecipientEmail != nil {
			email = *n.RecipientEmail
		}
		err = s.sender.SendEmail(ctx, email, title, n.Body)
	}

	if err != nil {
		s.log.Warn("notification delivery failed",
			zap.String("notification_id", n.ID),
			zap.String("channel", n.Channel),
			zap.Error(err))
		_ = s.repo.MarkFailed(ctx, n.ID)
		return
	}
	_ = s.repo.MarkSent(ctx, n.ID)
}

func (s *Service) UpdateStatus(ctx context.Context, id string, status string) error {
	return s.repo.UpdateStatus(ctx, id, strings.ToUpper(strings.TrimSpace(status)))
}

func (s *Service) UpdateStatusForUser(ctx context.Context, id string, userID string, status string) error {
	return s.repo.UpdateStatusForUser(ctx, id, userID, strings.ToUpper(strings.TrimSpace(status)))
}
