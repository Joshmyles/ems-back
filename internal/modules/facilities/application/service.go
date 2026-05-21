package application

import (
	"context"

	"dispatch/internal/modules/facilities/domain"
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

func (s *Service) ListFacilities(ctx context.Context, p platformdb.Pagination) (platformdb.PageResult[domain.Facility], error) {
	items, total, err := s.repo.ListFacilities(ctx, p)
	if err != nil {
		return platformdb.PageResult[domain.Facility]{}, err
	}
	return platformdb.PageResult[domain.Facility]{
		Items: items,
		Meta:  platformdb.NewPageMeta(p, total),
	}, nil
}

func (s *Service) GetFacility(ctx context.Context, uid string) (domain.Facility, error) {
	return s.repo.GetByUID(ctx, uid)
}

func (s *Service) CreateFacility(ctx context.Context, req CreateFacilityRequest) (domain.Facility, error) {
	f := domain.Facility{
		FacilityUID:  req.FacilityUID,
		SubcountyUID: req.SubcountyUID,
		Facility:     req.Facility,
		Level:        req.Level,
		Ownership:    req.Ownership,
	}
	return s.repo.Create(ctx, f)
}

func (s *Service) UpdateFacility(ctx context.Context, uid string, req UpdateFacilityRequest) (domain.Facility, error) {
	return s.repo.Update(ctx, uid, req)
}

func (s *Service) DeleteFacility(ctx context.Context, uid string) error {
	return s.repo.Delete(ctx, uid)
}

func (s *Service) SetFocalPerson(ctx context.Context, facilityUID string, req SetFocalPersonRequest) (domain.Facility, error) {
	return s.repo.SetFocalPerson(ctx, facilityUID, req.UserID)
}

func (s *Service) ClearFocalPerson(ctx context.Context, facilityUID string) (domain.Facility, error) {
	return s.repo.ClearFocalPerson(ctx, facilityUID)
}
