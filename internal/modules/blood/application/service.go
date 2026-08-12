package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"dispatch/internal/modules/blood/application/dto"
	blooddomain "dispatch/internal/modules/blood/domain"
	platformdb "dispatch/internal/platform/db"
	"dispatch/internal/platform/events"
)

type Service struct {
	repo Repository
	bus  events.Publisher
	log  *zap.Logger
}

func NewService(repo Repository, bus events.Publisher, log *zap.Logger) *Service {
	return &Service{repo: repo, bus: bus, log: log}
}

func (s *Service) ListRequisitions(ctx context.Context, p platformdb.Pagination) (platformdb.PageResult[blooddomain.BloodRequisition], error) {
	items, total, err := s.repo.ListRequisitions(ctx, p)
	if err != nil {
		return platformdb.PageResult[blooddomain.BloodRequisition]{}, err
	}
	return platformdb.PageResult[blooddomain.BloodRequisition]{Items: items, Meta: platformdb.NewPageMeta(p, total)}, nil
}

func (s *Service) RaiseRequisition(ctx context.Context, req dto.CreateBloodRequisitionRequest) (blooddomain.BloodRequisition, error) {
	bloodGroupID, err := s.repo.ResolveBloodGroupIDByCode(ctx, req.BloodGroupCode)
	if err != nil {
		return blooddomain.BloodRequisition{}, fmt.Errorf("resolve blood group: %w", err)
	}
	bloodProductID, err := s.repo.ResolveBloodProductIDByCode(ctx, req.BloodProductCode)
	if err != nil {
		return blooddomain.BloodRequisition{}, fmt.Errorf("resolve blood product: %w", err)
	}

	urgency := strings.ToUpper(strings.TrimSpace(req.UrgencyLevel))
	if urgency == "" {
		urgency = "EMERGENCY"
	}

	requisition := blooddomain.BloodRequisition{
		ID:                    uuid.NewString(),
		IncidentID:            req.IncidentID,
		RequestingFacilityID:  req.RequestingFacilityID,
		PatientName:           req.PatientName,
		PatientIdentifier:     req.PatientIdentifier,
		ClinicalSummary:       req.ClinicalSummary,
		Diagnosis:             req.Diagnosis,
		Indication:            req.Indication,
		ParitySummary:         req.ParitySummary,
		BloodGroupID:          bloodGroupID,
		BloodProductID:        bloodProductID,
		UnitsRequested:        req.UnitsRequested,
		UrgencyLevel:          urgency,
		Status:                "OPEN",
		ReporterPhone:         req.ReporterPhone,
		DestinationFacilityID: req.DestinationFacilityID,
		RequestedByUserID:     req.RequestedByUserID,
		ExpiresAt:             req.ExpiresAt,
	}

	created, err := s.repo.CreateRequisition(ctx, requisition)
	if err != nil {
		return blooddomain.BloodRequisition{}, err
	}

	_ = s.repo.CreateStatusLog(ctx, created.ID, "", created.Status, created.RequestedByUserID, "blood requisition raised")

	_ = s.bus.Publish(ctx, "blood.requisition.raised", events.Event{
		ID:          uuid.NewString(),
		Topic:       "blood.requisition.raised",
		AggregateID: created.ID,
		Type:        "blood.requisition.raised",
		OccurredAt:  time.Now().UTC(),
		Payload: map[string]any{
			"blood_requisition_id": created.ID,
			"blood_group_code":     req.BloodGroupCode,
			"blood_product_code":   req.BloodProductCode,
			"units_requested":      created.UnitsRequested,
			"urgency_level":        created.UrgencyLevel,
			"status":               created.Status,
		},
	})

	return created, nil
}

func (s *Service) BroadcastRequisition(ctx context.Context, requisitionID string, destLat, destLon *float64) ([]blooddomain.BloodBroadcastTarget, error) {
	req, err := s.repo.GetRequisitionByID(ctx, requisitionID)
	if err != nil {
		return nil, err
	}
	targets, err := s.repo.FindBroadcastTargets(ctx, req.BloodGroupID, req.BloodProductID, req.UnitsRequested, destLat, destLon, 20)
	if err != nil {
		return nil, err
	}
	msg := fmt.Sprintf("Blood request: %d unit(s) of %s %s needed urgently. %s",
		req.UnitsRequested, req.BloodProductCode, req.BloodGroupCode, req.ClinicalSummary)
	if len(targets) > 0 {
		if err := s.repo.CreateBroadcasts(ctx, requisitionID, msg, targets); err != nil {
			return nil, err
		}
	}
	prev := req.Status
	_ = s.repo.UpdateRequisitionStatus(ctx, requisitionID, "BROADCASTING")
	_ = s.repo.CreateStatusLog(ctx, requisitionID, prev, "BROADCASTING", req.RequestedByUserID, "broadcast sent to candidate sites")
	return targets, nil
}

func (s *Service) ListOffers(ctx context.Context, requisitionID string, p platformdb.Pagination) (platformdb.PageResult[blooddomain.BloodRequisitionOffer], error) {
	items, total, err := s.repo.ListOffers(ctx, requisitionID, p)
	if err != nil {
		return platformdb.PageResult[blooddomain.BloodRequisitionOffer]{}, err
	}
	return platformdb.PageResult[blooddomain.BloodRequisitionOffer]{Items: items, Meta: platformdb.NewPageMeta(p, total)}, nil
}

func (s *Service) AcceptOffer(ctx context.Context, requisitionID, offerID string, actorUserID *string) error {
	req, err := s.repo.GetRequisitionByID(ctx, requisitionID)
	if err != nil {
		return err
	}
	if err := s.repo.AcceptOffer(ctx, requisitionID, offerID); err != nil {
		return err
	}
	_ = s.repo.CreateStatusLog(ctx, requisitionID, req.Status, "MATCHED", actorUserID, "blood offer accepted")
	return s.repo.UpdateRequisitionStatus(ctx, requisitionID, "MATCHED")
}

func (s *Service) CreateOffer(ctx context.Context, req dto.CreateBloodOfferRequest) (blooddomain.BloodRequisitionOffer, error) {
	bloodGroupID, err := s.repo.ResolveBloodGroupIDByCode(ctx, req.BloodGroupCode)
	if err != nil {
		return blooddomain.BloodRequisitionOffer{}, fmt.Errorf("resolve blood group: %w", err)
	}
	bloodProductID, err := s.repo.ResolveBloodProductIDByCode(ctx, req.BloodProductCode)
	if err != nil {
		return blooddomain.BloodRequisitionOffer{}, fmt.Errorf("resolve blood product: %w", err)
	}

	offer := blooddomain.BloodRequisitionOffer{
		ID:                 uuid.NewString(),
		BloodRequisitionID: req.BloodRequisitionID,
		InventorySiteID:    req.InventorySiteID,
		BloodProductID:     bloodProductID,
		BloodGroupID:       bloodGroupID,
		UnitsOffered:       req.UnitsOffered,
		ReservedUntil:      req.ReservedUntil,
		Notes:              req.Notes,
		ContactPersonName:  req.ContactPersonName,
		ContactPhone:       req.ContactPhone,
		OfferedByUserID:    req.OfferedByUserID,
		Status:             "OFFERED",
	}
	created, err := s.repo.CreateOffer(ctx, offer)
	if err != nil {
		return blooddomain.BloodRequisitionOffer{}, err
	}
	_ = s.bus.Publish(ctx, "blood.offer.created", events.Event{
		ID:          uuid.NewString(),
		Topic:       "blood.offer.created",
		AggregateID: req.BloodRequisitionID,
		Type:        "blood.offer.created",
		OccurredAt:  time.Now().UTC(),
		Payload: map[string]any{
			"blood_requisition_id": req.BloodRequisitionID,
			"offer_id":             created.ID,
			"inventory_site_id":    created.InventorySiteID,
			"units_offered":        created.UnitsOffered,
		},
	})
	return created, nil
}

func (s *Service) AssignPickup(ctx context.Context, req dto.AssignBloodPickupRequest) (blooddomain.BloodTransportAssignment, error) {
	in := blooddomain.BloodTransportAssignment{
		ID:                      uuid.NewString(),
		BloodRequisitionID:      req.BloodRequisitionID,
		BloodRequisitionOfferID: req.BloodRequisitionOfferID,
		VehicleType:             strings.ToUpper(strings.TrimSpace(req.VehicleType)),
		AmbulanceID:             req.AmbulanceID,
		DispatchAssignmentID:    req.DispatchAssignmentID,
		AssignedDriverUserID:    req.AssignedDriverUserID,
		AssignedByUserID:        req.AssignedByUserID,
		PickupSiteID:            req.PickupSiteID,
		DestinationFacilityID:   req.DestinationFacilityID,
		Status:                  "ASSIGNED",
		Notes:                   req.Notes,
	}
	created, err := s.repo.CreateTransportAssignment(ctx, in)
	if err != nil {
		return blooddomain.BloodTransportAssignment{}, err
	}
	reqRow, _ := s.repo.GetRequisitionByID(ctx, req.BloodRequisitionID)
	_ = s.repo.UpdateRequisitionStatus(ctx, req.BloodRequisitionID, "PICKUP_ASSIGNED")
	_ = s.repo.CreateStatusLog(ctx, req.BloodRequisitionID, reqRow.Status, "PICKUP_ASSIGNED", req.AssignedByUserID, "transport assigned for blood pickup")
	_ = s.bus.Publish(ctx, "blood.pickup.assigned", events.Event{
		ID:          uuid.NewString(),
		Topic:       "blood.pickup.assigned",
		AggregateID: req.BloodRequisitionID,
		Type:        "blood.pickup.assigned",
		OccurredAt:  time.Now().UTC(),
		Payload: map[string]any{
			"blood_requisition_id":    req.BloodRequisitionID,
			"transport_assignment_id": created.ID,
			"vehicle_type":            req.VehicleType,
		},
	})
	return created, nil
}

func (s *Service) MarkCollected(ctx context.Context, assignmentID, requisitionID string, actorUserID *string) error {
	if err := s.repo.MarkTransportCollected(ctx, assignmentID); err != nil {
		return err
	}
	req, _ := s.repo.GetRequisitionByID(ctx, requisitionID)
	_ = s.repo.UpdateRequisitionStatus(ctx, requisitionID, "COLLECTED")
	return s.repo.CreateStatusLog(ctx, requisitionID, req.Status, "COLLECTED", actorUserID, "blood collected from source site")
}

func (s *Service) MarkDelivered(ctx context.Context, assignmentID, requisitionID string, actorUserID *string) error {
	if err := s.repo.MarkTransportDelivered(ctx, assignmentID); err != nil {
		return err
	}
	req, _ := s.repo.GetRequisitionByID(ctx, requisitionID)
	_ = s.repo.UpdateRequisitionStatus(ctx, requisitionID, "DELIVERED")
	return s.repo.CreateStatusLog(ctx, requisitionID, req.Status, "DELIVERED", actorUserID, "blood delivered to destination")
}

// Errors surfaced by requisition edit/delete.
var (
	ErrRequisitionForbidden = errors.New("you can only modify your own requisitions")
	ErrRequisitionLocked    = errors.New("requisition can no longer be modified at its current status")
)

// UpdateRequisition edits a requisition. Non-privileged actors (field medics)
// may only edit their own, and only while it is OPEN or BROADCASTING.
func (s *Service) UpdateRequisition(ctx context.Context, id string, req dto.UpdateBloodRequisitionRequest, actorUserID string, privileged bool) (blooddomain.BloodRequisition, error) {
	existing, err := s.repo.GetRequisitionByID(ctx, id)
	if err != nil {
		return blooddomain.BloodRequisition{}, err
	}
	if !privileged {
		if actorUserID == "" || existing.RequestedByUserID == nil || *existing.RequestedByUserID != actorUserID {
			return blooddomain.BloodRequisition{}, ErrRequisitionForbidden
		}
	}
	switch existing.Status {
	case "OPEN", "BROADCASTING":
	default:
		return blooddomain.BloodRequisition{}, ErrRequisitionLocked
	}
	return s.repo.UpdateRequisition(ctx, id, req)
}

// DeleteRequisition removes a requisition (cascades to broadcasts/offers).
// Blocked once a pickup workflow has started.
func (s *Service) DeleteRequisition(ctx context.Context, id, actorUserID string, privileged bool) error {
	existing, err := s.repo.GetRequisitionByID(ctx, id)
	if err != nil {
		return err
	}
	if !privileged {
		if actorUserID == "" || existing.RequestedByUserID == nil || *existing.RequestedByUserID != actorUserID {
			return ErrRequisitionForbidden
		}
	}
	switch existing.Status {
	case "OPEN", "BROADCASTING", "DECLINED", "CANCELLED", "EXPIRED":
	default:
		return ErrRequisitionLocked
	}
	return s.repo.DeleteRequisition(ctx, id)
}

// ErrDecisionNotAllowed guards the approve/decline transition.
var ErrDecisionNotAllowed = errors.New("a decision can only be made while the requisition is OPEN or BROADCASTING")

// DecideRequisition records the dispatcher/admin decision on a requisition:
// APPROVED (granted) or DECLINED. Logged in the status history with the actor.
func (s *Service) DecideRequisition(ctx context.Context, id, decision string, actorUserID *string, notes string) (blooddomain.BloodRequisition, error) {
	decision = strings.ToUpper(strings.TrimSpace(decision))
	if decision != "APPROVED" && decision != "DECLINED" {
		return blooddomain.BloodRequisition{}, fmt.Errorf("decision must be APPROVED or DECLINED")
	}
	existing, err := s.repo.GetRequisitionByID(ctx, id)
	if err != nil {
		return blooddomain.BloodRequisition{}, err
	}
	switch existing.Status {
	case "OPEN", "BROADCASTING":
	default:
		return blooddomain.BloodRequisition{}, ErrDecisionNotAllowed
	}
	if err := s.repo.UpdateRequisitionStatus(ctx, id, decision); err != nil {
		return blooddomain.BloodRequisition{}, err
	}
	logNote := notes
	if logNote == "" {
		if decision == "APPROVED" {
			logNote = "requisition approved (granted)"
		} else {
			logNote = "requisition declined"
		}
	}
	_ = s.repo.CreateStatusLog(ctx, id, existing.Status, decision, actorUserID, logNote)
	return s.repo.GetRequisitionByID(ctx, id)
}
