package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"math"
	"time"

	"dispatch/internal/modules/fuel/domain"
	platformdb "dispatch/internal/platform/db"

	"go.uber.org/zap"
)

// roundMoney rounds a monetary amount to 2 decimal places.
func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

// Sentinel errors surfaced to the public QR endpoints.
var (
	ErrFuelLogNotFound       = errors.New("fuel log not found")
	ErrAlreadyConfirmed      = errors.New("fuel dispense already confirmed")
	ErrFundingSourceNotFound = errors.New("funding source not found")
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

// Summary returns status counts, recent spend and trends for the fuel board.
func (s *Service) Summary(ctx context.Context, driverUserID *string) (domain.FuelLogSummary, error) {
	return s.repo.Summarize(ctx, driverUserID)
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
		AmbulanceID:     req.AmbulanceID,
		FuelType:        req.FuelType,
		Liters:          req.Liters,
		UnitCost:        req.UnitCost,
		Cost:            req.Cost,
		OdometerKM:      req.OdometerKM,
		StationName:     req.StationName,
		FilledAt:        filledAt,
		FilledBy:        filledByUserID,
		Notes:           req.Notes,
		PublicToken:     token,
		FundingSourceID: req.FundingSourceID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	// Total cost is derived from the user-entered unit cost: cost = liters * unit_cost.
	if req.UnitCost != nil {
		total := roundMoney(req.Liters * *req.UnitCost)
		in.Cost = &total
	}
	return s.repo.Create(ctx, in)
}

func (s *Service) Update(ctx context.Context, id string, req UpdateFuelLogRequest) (domain.FuelLog, error) {
	// When the unit cost is provided, re-derive the total cost from the
	// effective liters (the new value when supplied, otherwise the stored one).
	if req.UnitCost != nil {
		liters := 0.0
		if req.Liters != nil {
			liters = *req.Liters
		} else {
			existing, err := s.repo.GetByID(ctx, id, nil)
			if err != nil {
				return domain.FuelLog{}, err
			}
			liters = existing.Liters
		}
		total := roundMoney(liters * *req.UnitCost)
		req.Cost = &total
	}
	return s.repo.Update(ctx, id, req)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ListFundingSources returns all fuel funding sources with computed
// spent/remaining balances.
func (s *Service) ListFundingSources(ctx context.Context) ([]domain.FundingSource, error) {
	return s.repo.ListFundingSources(ctx)
}

func (s *Service) CreateFundingSource(ctx context.Context, req CreateFundingSourceRequest) (domain.FundingSource, error) {
	in := domain.FundingSource{
		OrganisationName: req.OrganisationName,
		Amount:           roundMoney(req.Amount),
		Notes:            req.Notes,
	}
	if req.FundingDate != nil {
		d, err := time.Parse("2006-01-02", *req.FundingDate)
		if err != nil {
			return domain.FundingSource{}, err
		}
		in.FundingDate = d
	}
	return s.repo.CreateFundingSource(ctx, in)
}

func (s *Service) GetFundingSource(ctx context.Context, id string) (domain.FundingSource, error) {
	return s.repo.GetFundingSource(ctx, id)
}

func (s *Service) UpdateFundingSource(ctx context.Context, id string, req UpdateFundingSourceRequest) (domain.FundingSource, error) {
	if req.Amount != nil {
		rounded := roundMoney(*req.Amount)
		req.Amount = &rounded
	}
	return s.repo.UpdateFundingSource(ctx, id, req)
}

func (s *Service) DeleteFundingSource(ctx context.Context, id string) error {
	return s.repo.DeleteFundingSource(ctx, id)
}

// TopUpFundingSource adds money to a funding source and returns the updated
// source (with recomputed totals) together with the recorded top-up.
func (s *Service) TopUpFundingSource(ctx context.Context, id string, req CreateFundingTopupRequest, createdByUserID *string) (domain.FundingSource, domain.FundingTopup, error) {
	// Ensure the source exists before recording money against it.
	if _, err := s.repo.GetFundingSource(ctx, id); err != nil {
		return domain.FundingSource{}, domain.FundingTopup{}, ErrFundingSourceNotFound
	}

	in := domain.FundingTopup{
		FundingSourceID: id,
		Amount:          roundMoney(req.Amount),
		Notes:           req.Notes,
		CreatedBy:       createdByUserID,
	}
	if req.TopupDate != nil {
		d, err := time.Parse("2006-01-02", *req.TopupDate)
		if err != nil {
			return domain.FundingSource{}, domain.FundingTopup{}, err
		}
		in.TopupDate = d
	}

	topup, err := s.repo.AddFundingTopup(ctx, in)
	if err != nil {
		return domain.FundingSource{}, domain.FundingTopup{}, err
	}
	source, err := s.repo.GetFundingSource(ctx, id)
	if err != nil {
		return domain.FundingSource{}, domain.FundingTopup{}, err
	}
	return source, topup, nil
}

func (s *Service) ListFundingTopups(ctx context.Context, id string) ([]domain.FundingTopup, error) {
	return s.repo.ListFundingTopups(ctx, id)
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
