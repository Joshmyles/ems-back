package application

import (
	"context"

	"dispatch/internal/modules/blood/application/dto"
	blooddomain "dispatch/internal/modules/blood/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository interface {
	ResolveBloodGroupIDByCode(ctx context.Context, code string) (string, error)
	ResolveBloodProductIDByCode(ctx context.Context, code string) (string, error)
	// ResolveIncidentID accepts an incident UUID or incident number.
	ResolveIncidentID(ctx context.Context, ref string) (string, error)
	// ResolveInventorySiteID accepts a blood inventory site ID or a facility
	// ID; for a facility it returns (creating if needed) its inventory site.
	ResolveInventorySiteID(ctx context.Context, ref string) (string, error)
	CreateRequisition(ctx context.Context, req blooddomain.BloodRequisition) (blooddomain.BloodRequisition, error)
	ListRequisitions(ctx context.Context, p platformdb.Pagination) ([]blooddomain.BloodRequisition, int64, error)
	GetRequisitionByID(ctx context.Context, id string) (blooddomain.BloodRequisition, error)
	UpdateRequisition(ctx context.Context, id string, req dto.UpdateBloodRequisitionRequest) (blooddomain.BloodRequisition, error)
	DeleteRequisition(ctx context.Context, id string) error
	UpdateRequisitionStatus(ctx context.Context, id, status string) error
	CreateBroadcasts(ctx context.Context, requisitionID string, message string, targets []blooddomain.BloodBroadcastTarget) error
	FindBroadcastTargets(ctx context.Context, bloodGroupID, bloodProductID string, unitsRequested int, destLat, destLon *float64, limit int) ([]blooddomain.BloodBroadcastTarget, error)
	CreateOffer(ctx context.Context, offer blooddomain.BloodRequisitionOffer) (blooddomain.BloodRequisitionOffer, error)
	ListOffers(ctx context.Context, requisitionID string, p platformdb.Pagination) ([]blooddomain.BloodRequisitionOffer, int64, error)
	AcceptOffer(ctx context.Context, requisitionID, offerID string) error
	CreateTransportAssignment(ctx context.Context, in blooddomain.BloodTransportAssignment) (blooddomain.BloodTransportAssignment, error)
	MarkTransportCollected(ctx context.Context, assignmentID, requisitionID string) error
	MarkTransportDelivered(ctx context.Context, assignmentID, requisitionID string) error
	CreateStatusLog(ctx context.Context, requisitionID, prevStatus, newStatus string, actorUserID *string, notes string) error
	SummarizeRequisitions(ctx context.Context) (blooddomain.BloodRequisitionSummary, error)
	GetRequisitionTracking(ctx context.Context, requisitionID string) (blooddomain.BloodRequisitionTracking, error)
}
