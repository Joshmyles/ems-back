package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"dispatch/internal/modules/fuel/domain"
	platformdb "dispatch/internal/platform/db"

	"go.uber.org/zap"
)

// Sentinel errors surfaced to the public QR endpoints.
var (
	ErrFuelLogNotFound  = errors.New("fuel log not found")
	ErrAlreadyConfirmed = errors.New("fuel dispense already confirmed")
)

type Service struct {
	repo Repository
	log  *zap.Logger
}

func NewService(repo Repository, log *zap.Logger) *Service {
	return &Service{repo: repo, log: log}
}

// generatePublicToken returns a random, URL-safe token for the QR link.
func generatePublicToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// List returns fuel logs. When driverUserID is non-nil, the result is scoped
// to fuel logs for ambulances the user is the active driver of.
func (s *Service) List(ctx context.Context, p platformdb.Pagination, driverUserID *string) ([]domain.FuelLog, int64, error) {
	return s.repo.List(ctx, p, driverUserID)
}

// Get returns a single fuel log. When driverUserID is non-nil, the lookup is
// scoped so a driver cannot read fuel logs for ambulances they are not on.
func (s *Service) Get(ctx context.Context, id string, driverUserID *string) (domain.FuelLog, error) {
	return s.repo.GetByID(ctx, id, driverUserID)
}

func (s *Service) Create(ctx context.Context, req CreateFuelLogRequest, filledByUserID *string) (domain.FuelLog, error) {
	now := time.Now()
	filledAt := now
	if req.FilledAt != nil {
		filledAt = *req.FilledAt
	}

	token, err := generatePublicToken()
	if err != nil {
		return domain.FuelLog{}, err
	}

	in := domain.FuelLog{
		AmbulanceID: req.AmbulanceID,
		FuelType:    req.FuelType,
		Liters:      req.Liters,
		Cost:        req.Cost,
		OdometerKM:  req.OdometerKM,
		StationName: req.StationName,
		FilledAt:    filledAt,
		FilledBy:    filledByUserID,
		Notes:       req.Notes,
		PublicToken: token,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateFuelLogRequest) (domain.FuelLog, error) {
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// GetPublic returns the QR-scanned view of a fuel log by its public token.
func (s *Service) GetPublic(ctx context.Context, token string) (domain.FuelLogPublicView, error) {
	view, err := s.repo.GetPublicByToken(ctx, token)
	if err != nil {
		return domain.FuelLogPublicView{}, ErrFuelLogNotFound
	}
	return view, nil
}

// ConfirmDispense records the fuel station attendant's confirmation. Once a
// fuel log has been confirmed it is locked and cannot be confirmed again.
func (s *Service) ConfirmDispense(ctx context.Context, token string, req ConfirmFuelDispenseRequest) (domain.FuelLogPublicView, error) {
	view, err := s.repo.GetPublicByToken(ctx, token)
	if err != nil {
		return domain.FuelLogPublicView{}, ErrFuelLogNotFound
	}
	if view.FuelLog.DispenseConfirmed {
		return domain.FuelLogPublicView{}, ErrAlreadyConfirmed
	}

	rows, err := s.repo.ConfirmDispense(ctx, token, req)
	if err != nil {
		return domain.FuelLogPublicView{}, err
	}
	if rows == 0 {
		// Lost a race with another confirmation.
		return domain.FuelLogPublicView{}, ErrAlreadyConfirmed
	}

	return s.repo.GetPublicByToken(ctx, token)
}
