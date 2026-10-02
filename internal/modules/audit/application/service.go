package application

import (
	"context"
	"time"

	"go.uber.org/zap"

	auditdomain "dispatch/internal/modules/audit/domain"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/shared/types"
)

type Repository interface {
	Insert(ctx context.Context, e types.AuditEntry) error
	List(ctx context.Context, params auditdomain.ListParams) ([]auditdomain.Entry, int64, error)
}

// Service is both the write-side AuditRecorder (implements types.AuditRecorder)
// and the read-side query service used by the audit API.
type Service struct {
	repo Repository
	log  *zap.Logger
}

func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// Record persists an audit entry best-effort. It never blocks the caller and
// never returns an error: a failed write is logged and dropped, because losing
// an audit line must not break the action being audited.
//
// The write runs on a detached context so it still completes if the request
// context is cancelled (e.g. the client disconnects right after the action).
func (s *Service) Record(ctx context.Context, entry types.AuditEntry) {
	if s == nil || s.repo == nil {
		return
	}
	if entry.Status == "" {
		entry.Status = types.AuditStatusSuccess
	}

	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		if err := s.repo.Insert(writeCtx, entry); err != nil && s.log != nil {
			s.log.Warn("audit record failed",
				zap.String("action", entry.Action),
				zap.String("entity_type", entry.EntityType),
				zap.Error(err),
			)
		}
	}()
}

func (s *Service) List(ctx context.Context, params auditdomain.ListParams) (platformdb.PageResult[auditdomain.Entry], error) {
	items, total, err := s.repo.List(ctx, params)
	if err != nil {
		return platformdb.PageResult[auditdomain.Entry]{}, err
	}
	if items == nil {
		items = []auditdomain.Entry{}
	}
	return platformdb.PageResult[auditdomain.Entry]{
		Items: items,
		Meta:  platformdb.NewPageMeta(params.Pagination, total),
	}, nil
}

var _ types.AuditRecorder = (*Service)(nil)
