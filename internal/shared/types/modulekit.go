package types

import (
	"context"

	"dispatch/internal/platform/config"
	"dispatch/internal/platform/events"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// NotificationSender delivers notifications to their final channel (FCM
// push, SMS, email). Implemented by the notifications infrastructure Sender.
type NotificationSender interface {
	SendSMS(ctx context.Context, to string, body string) error
	SendEmail(ctx context.Context, to, subject, body string) error
	SendPush(ctx context.Context, userID string, title, body string) error
}

type ModuleDeps struct {
	Router *gin.RouterGroup
	DB     *pgxpool.Pool
	Redis  *redis.Client
	Logger *zap.Logger
	Bus    events.Publisher
	Config config.Config
	// PushSender is nil when Firebase credentials are not configured; the
	// notifications service then leaves rows PENDING instead of sending.
	PushSender NotificationSender
	// Audit records security-relevant actions to the audit trail. Never nil —
	// bootstrap installs a real recorder, falling back to NoopAuditRecorder.
	Audit AuditRecorder
}
