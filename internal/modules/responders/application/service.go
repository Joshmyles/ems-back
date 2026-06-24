package application

import (
	"context"

	"dispatch/internal/modules/responders/domain"
	platformdb "dispatch/internal/platform/db"

	"go.uber.org/zap"
)

type Service struct {
	repo Repository
	log  *zap.Logger
}

func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// ListResponders returns paginated responders.
func (s *Service) ListResponders(ctx context.Context, p platformdb.Pagination) (platformdb.PageResult[domain.Responder], error) {
	items, total, err := s.repo.ListResponders(ctx, p)
	if err != nil {
		return platformdb.PageResult[domain.Responder]{}, err
	}
	return platformdb.PageResult[domain.Responder]{
		Items: items,
		Meta:  platformdb.NewPageMeta(p, total),
	}, nil
}

// GetResponder returns a single responder by ambulance id.
func (s *Service) GetResponder(ctx context.Context, id string) (domain.Responder, error) {
	return s.repo.GetByID(ctx, id)
}
