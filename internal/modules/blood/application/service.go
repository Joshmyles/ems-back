package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

func (s *Service) SummarizeRequisitions(ctx context.Context) (blooddomain.BloodRequisitionSummary, error) {
	return s.repo.SummarizeRequisitions(ctx)
}

// GetRequisitionTracking returns the status history and live pickup leg.
func (s *Service) GetRequisitionTracking(ctx context.Context, requisitionID string) (blooddomain.BloodRequisitionTracking, error) {
	if _, err := s.getRequisition(ctx, requisitionID); err != nil {
		return blooddomain.BloodRequisitionTracking{}, err
	}
	return s.repo.GetRequisitionTracking(ctx, requisitionID)
}

// ErrInvalidInput marks client mistakes (unknown codes, bad references) so the
// handler can answer 400 instead of 500.
var ErrInvalidInput = errors.New("invalid input")

// ErrNotFound is returned when a requisition, offer or assignment is missing.
var ErrNotFound = errors.New("not found")

var validUrgencyLevels = map[string]struct{}{"EMERGENCY": {}, "URGENT": {}, "ROUTINE": {}}

var validVehicleTypes = map[string]struct{}{"AMBULANCE": {}, "PICKUP": {}, "MOTORCYCLE": {}, "OTHER": {}}

// blankToNil turns "" / whitespace into nil so optional UUID columns get NULL
// rather than an invalid-uuid error. Clients often send "" for unset fields.
func blankToNil(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *Service) resolveBloodCodes(ctx context.Context, groupCode, productCode string) (string, string, error) {
	groupID, err := s.repo.ResolveBloodGroupIDByCode(ctx, strings.TrimSpace(groupCode))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("%w: unknown blood group %q", ErrInvalidInput, groupCode)
	}
	if err != nil {
		return "", "", fmt.Errorf("resolve blood group: %w", err)
	}
	productID, err := s.repo.ResolveBloodProductIDByCode(ctx, strings.TrimSpace(productCode))
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", fmt.Errorf("%w: unknown blood product %q", ErrInvalidInput, productCode)
	}
	if err != nil {
		return "", "", fmt.Errorf("resolve blood product: %w", err)
	}
	return groupID, productID, nil
}

// resolveSite maps a blood inventory site ID or a facility ID to an inventory
// site ID. nil in, nil out.
func (s *Service) resolveSite(ctx context.Context, ref *string) (*string, error) {
	if ref == nil {
		return nil, nil
	}
	id, err := s.repo.ResolveInventorySiteID(ctx, *ref)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: unknown inventory site or facility %q", ErrInvalidInput, *ref)
	}
	if err != nil {
		return nil, fmt.Errorf("resolve inventory site: %w", err)
	}
	return &id, nil
}

func (s *Service) getRequisition(ctx context.Context, id string) (blooddomain.BloodRequisition, error) {
	req, err := s.repo.GetRequisitionByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return blooddomain.BloodRequisition{}, fmt.Errorf("%w: blood requisition %s", ErrNotFound, id)
	}
	return req, err
}

func (s *Service) RaiseRequisition(ctx context.Context, req dto.CreateBloodRequisitionRequest) (blooddomain.BloodRequisition, error) {
	bloodGroupID, bloodProductID, err := s.resolveBloodCodes(ctx, req.BloodGroupCode, req.BloodProductCode)
	if err != nil {
		return blooddomain.BloodRequisition{}, err
	}

	urgency := strings.ToUpper(strings.TrimSpace(req.UrgencyLevel))
	if urgency == "" {
		urgency = "EMERGENCY"
	}
	if _, ok := validUrgencyLevels[urgency]; !ok {
		return blooddomain.BloodRequisition{}, fmt.Errorf("%w: urgency_level must be EMERGENCY, URGENT or ROUTINE", ErrInvalidInput)
	}

	// The incident can be referenced by its UUID or its human-readable number.
	incidentID := blankToNil(req.IncidentID)
	if incidentID != nil {
		resolved, err := s.repo.ResolveIncidentID(ctx, *incidentID)
		if errors.Is(err, pgx.ErrNoRows) {
			return blooddomain.BloodRequisition{}, fmt.Errorf("%w: unknown incident %q", ErrInvalidInput, *incidentID)
		}
		if err != nil {
			return blooddomain.BloodRequisition{}, fmt.Errorf("resolve incident: %w", err)
		}
		incidentID = &resolved
	}

	requisition := blooddomain.BloodRequisition{
		ID:                    uuid.NewString(),
		IncidentID:            incidentID,
		RequestingFacilityID:  blankToNil(req.RequestingFacilityID),
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
		DestinationFacilityID: blankToNil(req.DestinationFacilityID),
		RequestedByUserID:     blankToNil(req.RequestedByUserID),
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
	req, err := s.getRequisition(ctx, requisitionID)
	if err != nil {
		return nil, err
	}
	targets, err := s.repo.FindBroadcastTargets(ctx, req.BloodGroupID, req.BloodProductID, req.UnitsRequested, destLat, destLon, 20)
	if err != nil {
		return nil, err
	}
	if targets == nil {
		targets = []blooddomain.BloodBroadcastTarget{}
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

// Offer acceptance guards: without them, re-clicking Accept on a delivered
// requisition pulled it back to MATCHED.
var (
	ErrOfferNotOpen     = errors.New("this offer is no longer open for acceptance")
	ErrAcceptNotAllowed = errors.New("offers can only be accepted before a pickup is assigned")
)

func (s *Service) AcceptOffer(ctx context.Context, requisitionID, offerID string, actorUserID *string) error {
	req, err := s.getRequisition(ctx, requisitionID)
	if err != nil {
		return err
	}
	switch req.Status {
	case "OPEN", "APPROVED", "BROADCASTING", "MATCHED":
	default:
		return ErrAcceptNotAllowed
	}
	if err := s.repo.AcceptOffer(ctx, req.ID, offerID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: offer %s for this requisition", ErrNotFound, offerID)
		}
		return err
	}
	_ = s.repo.CreateStatusLog(ctx, req.ID, req.Status, "MATCHED", actorUserID, "blood offer accepted")
	return nil
}

func (s *Service) CreateOffer(ctx context.Context, req dto.CreateBloodOfferRequest) (blooddomain.BloodRequisitionOffer, error) {
	requisition, err := s.getRequisition(ctx, req.BloodRequisitionID)
	if err != nil {
		return blooddomain.BloodRequisitionOffer{}, err
	}
	bloodGroupID, bloodProductID, err := s.resolveBloodCodes(ctx, req.BloodGroupCode, req.BloodProductCode)
	if err != nil {
		return blooddomain.BloodRequisitionOffer{}, err
	}
	// The UI picks a facility; resolve it to (or create) its blood inventory site.
	siteID, err := s.resolveSite(ctx, blankToNil(&req.InventorySiteID))
	if err != nil {
		return blooddomain.BloodRequisitionOffer{}, err
	}
	if siteID == nil {
		return blooddomain.BloodRequisitionOffer{}, fmt.Errorf("%w: inventory_site_id is required", ErrInvalidInput)
	}

	offer := blooddomain.BloodRequisitionOffer{
		ID:                 uuid.NewString(),
		BloodRequisitionID: requisition.ID,
		InventorySiteID:    *siteID,
		BloodProductID:     bloodProductID,
		BloodGroupID:       bloodGroupID,
		UnitsOffered:       req.UnitsOffered,
		ReservedUntil:      req.ReservedUntil,
		Notes:              req.Notes,
		ContactPersonName:  req.ContactPersonName,
		ContactPhone:       req.ContactPhone,
		OfferedByUserID:    blankToNil(req.OfferedByUserID),
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
	reqRow, err := s.getRequisition(ctx, req.BloodRequisitionID)
	if err != nil {
		return blooddomain.BloodTransportAssignment{}, err
	}
	vehicleType := strings.ToUpper(strings.TrimSpace(req.VehicleType))
	if _, ok := validVehicleTypes[vehicleType]; !ok {
		return blooddomain.BloodTransportAssignment{}, fmt.Errorf("%w: vehicle_type must be AMBULANCE, PICKUP, MOTORCYCLE or OTHER", ErrInvalidInput)
	}
	pickupSiteID, err := s.resolveSite(ctx, blankToNil(req.PickupSiteID))
	if err != nil {
		return blooddomain.BloodTransportAssignment{}, err
	}

	in := blooddomain.BloodTransportAssignment{
		ID:                      uuid.NewString(),
		BloodRequisitionID:      reqRow.ID,
		BloodRequisitionOfferID: blankToNil(req.BloodRequisitionOfferID),
		VehicleType:             vehicleType,
		AmbulanceID:             blankToNil(req.AmbulanceID),
		DispatchAssignmentID:    blankToNil(req.DispatchAssignmentID),
		AssignedDriverUserID:    blankToNil(req.AssignedDriverUserID),
		AssignedByUserID:        blankToNil(req.AssignedByUserID),
		PickupSiteID:            pickupSiteID,
		DestinationFacilityID:   blankToNil(req.DestinationFacilityID),
		Status:                  "ASSIGNED",
		Notes:                   req.Notes,
	}
	created, err := s.repo.CreateTransportAssignment(ctx, in)
	if err != nil {
		return blooddomain.BloodTransportAssignment{}, err
	}
	_ = s.repo.UpdateRequisitionStatus(ctx, reqRow.ID, "PICKUP_ASSIGNED")
	_ = s.repo.CreateStatusLog(ctx, reqRow.ID, reqRow.Status, "PICKUP_ASSIGNED", req.AssignedByUserID, "transport assigned for blood pickup")
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
	req, err := s.getRequisition(ctx, requisitionID)
	if err != nil {
		return err
	}
	if err := s.repo.MarkTransportCollected(ctx, assignmentID, req.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: pickup assignment %s for this requisition", ErrNotFound, assignmentID)
		}
		return err
	}
	if err := s.repo.UpdateRequisitionStatus(ctx, req.ID, "COLLECTED"); err != nil {
		return err
	}
	return s.repo.CreateStatusLog(ctx, req.ID, req.Status, "COLLECTED", actorUserID, "blood collected from source site")
}

func (s *Service) MarkDelivered(ctx context.Context, assignmentID, requisitionID string, actorUserID *string) error {
	req, err := s.getRequisition(ctx, requisitionID)
	if err != nil {
		return err
	}
	if err := s.repo.MarkTransportDelivered(ctx, assignmentID, req.ID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("%w: pickup assignment %s for this requisition", ErrNotFound, assignmentID)
		}
		return err
	}
	if err := s.repo.UpdateRequisitionStatus(ctx, req.ID, "DELIVERED"); err != nil {
		return err
	}
	return s.repo.CreateStatusLog(ctx, req.ID, req.Status, "DELIVERED", actorUserID, "blood delivered to destination")
}

// Errors surfaced by requisition edit/delete.
var (
	ErrRequisitionForbidden = errors.New("you can only modify your own requisitions")
	ErrRequisitionLocked    = errors.New("requisition can no longer be modified at its current status")
)

// UpdateRequisition edits a requisition. Non-privileged actors (field medics)
// may only edit their own, and only while it is OPEN or BROADCASTING.
func (s *Service) UpdateRequisition(ctx context.Context, id string, req dto.UpdateBloodRequisitionRequest, actorUserID string, privileged bool) (blooddomain.BloodRequisition, error) {
	existing, err := s.getRequisition(ctx, id)
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
	// Unknown codes would otherwise resolve to NULL and hit a NOT NULL violation.
	if req.BloodGroupCode != nil || req.BloodProductCode != nil {
		group, product := existing.BloodGroupCode, existing.BloodProductCode
		if req.BloodGroupCode != nil {
			group = *req.BloodGroupCode
		}
		if req.BloodProductCode != nil {
			product = *req.BloodProductCode
		}
		if _, _, err := s.resolveBloodCodes(ctx, group, product); err != nil {
			return blooddomain.BloodRequisition{}, err
		}
	}
	return s.repo.UpdateRequisition(ctx, existing.ID, req)
}

// DeleteRequisition removes a requisition (cascades to broadcasts/offers).
// Blocked once a pickup workflow has started.
func (s *Service) DeleteRequisition(ctx context.Context, id, actorUserID string, privileged bool) error {
	existing, err := s.getRequisition(ctx, id)
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
	existing, err := s.getRequisition(ctx, id)
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
