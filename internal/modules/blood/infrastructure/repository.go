package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	bloodapp "dispatch/internal/modules/blood/application"
	bloodappdto "dispatch/internal/modules/blood/application/dto"
	blooddomain "dispatch/internal/modules/blood/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

var _ bloodapp.Repository = (*Repository)(nil)

func (r *Repository) ResolveBloodGroupIDByCode(ctx context.Context, code string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM blood_groups WHERE UPPER(code)=UPPER($1)`, code).Scan(&id)
	return id, err
}

func (r *Repository) ResolveBloodProductIDByCode(ctx context.Context, code string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM blood_products WHERE UPPER(code)=UPPER($1)`, code).Scan(&id)
	return id, err
}

func (r *Repository) ResolveIncidentID(ctx context.Context, ref string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id::text FROM incidents
		WHERE id::text = LOWER($1) OR UPPER(incident_number) = UPPER($1)
		LIMIT 1`, strings.TrimSpace(ref)).Scan(&id)
	return id, err
}

func (r *Repository) ResolveInventorySiteID(ctx context.Context, ref string) (string, error) {
	ref = strings.ToLower(strings.TrimSpace(ref))
	var id string
	err := platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `SELECT id::text FROM blood_inventory_sites WHERE id::text = $1`, ref).Scan(&id)
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// Serialise get-or-create per facility so concurrent offers don't
		// create duplicate sites.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('blood_inventory_site:' || $1))`, ref); err != nil {
			return err
		}
		err = tx.QueryRow(ctx, `
			SELECT id::text FROM blood_inventory_sites
			WHERE site_type = 'FACILITY' AND facility_id::text = $1
			ORDER BY is_active DESC, created_at
			LIMIT 1`, ref).Scan(&id)
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return tx.QueryRow(ctx, `
			INSERT INTO blood_inventory_sites (
				site_type, facility_id, name, district_id, contact_phone, latitude, longitude, location
			)
			SELECT 'FACILITY', f.id, f.name, f.district_id, f.phone, f.latitude, f.longitude, f.location
			FROM ref_facilities f
			WHERE f.id::text = $1
			RETURNING id::text`, ref).Scan(&id)
	})
	return id, err
}

// requisitionSelect is shared by the single and list reads. The last two
// columns expose the accepted offer and latest live pickup assignment so
// clients can drive collect/deliver without remembering IDs locally.
const requisitionSelect = `
	SELECT br.id, br.incident_id, br.requesting_facility_id, COALESCE(br.patient_name,''), COALESCE(br.patient_identifier,''),
	       br.clinical_summary, COALESCE(br.diagnosis,''), COALESCE(br.indication,''), COALESCE(br.parity_summary,''),
	       br.blood_group_id, bg.code, br.blood_product_id, bp.code, br.units_requested, br.urgency_level, br.status,
	       COALESCE(br.reporter_phone,''), br.destination_facility_id, br.requested_by_user_id, br.created_at, br.updated_at, br.expires_at,
	       COALESCE(rf.name,''), COALESCE(df.name,''), COALESCE(TRIM(CONCAT_WS(' ', ru.first_name, ru.last_name, ru.other_name)),''), COALESCE(inc.incident_number,''),
	       (SELECT bro.id::text FROM blood_requisition_offers bro
	         WHERE bro.blood_requisition_id = br.id AND bro.status = 'ACCEPTED'
	         ORDER BY bro.updated_at DESC LIMIT 1),
	       (SELECT bta.id::text FROM blood_transport_assignments bta
	         WHERE bta.blood_requisition_id = br.id AND bta.status <> 'CANCELLED'
	         ORDER BY bta.assigned_at DESC LIMIT 1)
	FROM blood_requisitions br
	JOIN blood_groups bg ON bg.id = br.blood_group_id
	JOIN blood_products bp ON bp.id = br.blood_product_id
	LEFT JOIN ref_facilities rf ON rf.id = br.requesting_facility_id
	LEFT JOIN ref_facilities df ON df.id = br.destination_facility_id
	LEFT JOIN users ru ON ru.id = br.requested_by_user_id
	LEFT JOIN incidents inc ON inc.id = br.incident_id`

func scanRequisition(row pgx.Row) (blooddomain.BloodRequisition, error) {
	var out blooddomain.BloodRequisition
	err := row.Scan(
		&out.ID, &out.IncidentID, &out.RequestingFacilityID, &out.PatientName, &out.PatientIdentifier,
		&out.ClinicalSummary, &out.Diagnosis, &out.Indication, &out.ParitySummary,
		&out.BloodGroupID, &out.BloodGroupCode, &out.BloodProductID, &out.BloodProductCode,
		&out.UnitsRequested, &out.UrgencyLevel, &out.Status,
		&out.ReporterPhone, &out.DestinationFacilityID, &out.RequestedByUserID,
		&out.CreatedAt, &out.UpdatedAt, &out.ExpiresAt,
		&out.RequestingFacilityName, &out.DestinationFacilityName, &out.RequestedByUserName, &out.IncidentNumber,
		&out.AcceptedOfferID, &out.PickupAssignmentID,
	)
	return out, err
}

func (r *Repository) CreateRequisition(ctx context.Context, req blooddomain.BloodRequisition) (blooddomain.BloodRequisition, error) {
	query := `
	INSERT INTO blood_requisitions (
		id, incident_id, requesting_facility_id, patient_name, patient_identifier,
		clinical_summary, diagnosis, indication, parity_summary,
		blood_group_id, blood_product_id, units_requested, urgency_level, status,
		reporter_phone, destination_facility_id, destination_lat, destination_lon,
		requested_by_user_id, expires_at
	)
	VALUES (
		$1,$2,$3,$4,$5,
		$6,$7,$8,$9,
		$10,$11,$12,$13,$14,
		$15,$16,NULL,NULL,
		$17,$18
	)
	RETURNING created_at, updated_at`
	err := r.db.QueryRow(ctx, query,
		req.ID, req.IncidentID, req.RequestingFacilityID, req.PatientName, req.PatientIdentifier,
		req.ClinicalSummary, req.Diagnosis, req.Indication, req.ParitySummary,
		req.BloodGroupID, req.BloodProductID, req.UnitsRequested, req.UrgencyLevel, req.Status,
		req.ReporterPhone, req.DestinationFacilityID,
		req.RequestedByUserID, req.ExpiresAt,
	).Scan(&req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return blooddomain.BloodRequisition{}, err
	}
	return r.GetRequisitionByID(ctx, req.ID)
}

func (r *Repository) GetRequisitionByID(ctx context.Context, id string) (blooddomain.BloodRequisition, error) {
	// Compare as text so a malformed id is a clean "not found", not a cast error.
	return scanRequisition(r.db.QueryRow(ctx, requisitionSelect+` WHERE br.id::text = LOWER($1)`, strings.TrimSpace(id)))
}

func splitUpper(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.ToUpper(strings.TrimSpace(part)); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// liveRequisitionStatuses still need coordination work.
var liveRequisitionStatuses = map[string]bool{
	"OPEN": true, "APPROVED": true, "BROADCASTING": true,
	"MATCHED": true, "PICKUP_ASSIGNED": true, "COLLECTED": true,
}

func (r *Repository) SummarizeRequisitions(ctx context.Context) (blooddomain.BloodRequisitionSummary, error) {
	out := blooddomain.BloodRequisitionSummary{
		ByStatus:        map[string]int64{},
		ByUrgency:       map[string]int64{},
		ByStatusUrgency: map[string]map[string]int64{},
	}
	rows, err := r.db.Query(ctx, `
		SELECT status, urgency_level, COUNT(1), COALESCE(SUM(units_requested), 0)
		FROM blood_requisitions
		GROUP BY status, urgency_level`)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	for rows.Next() {
		var status, urgency string
		var count, units int64
		if err := rows.Scan(&status, &urgency, &count, &units); err != nil {
			return out, err
		}
		out.Total += count
		out.ByStatus[status] += count
		if out.ByStatusUrgency[status] == nil {
			out.ByStatusUrgency[status] = map[string]int64{}
		}
		out.ByStatusUrgency[status][urgency] += count
		if liveRequisitionStatuses[status] {
			out.ByUrgency[urgency] += count
			out.LiveTotal += count
			out.UnitsPending += units
			if urgency == "EMERGENCY" {
				out.LiveEmergency += count
			}
		}
	}
	return out, rows.Err()
}

func (r *Repository) GetRequisitionTracking(ctx context.Context, requisitionID string) (blooddomain.BloodRequisitionTracking, error) {
	out := blooddomain.BloodRequisitionTracking{Events: []blooddomain.BloodRequisitionEvent{}}

	rows, err := r.db.Query(ctx, `
		SELECT l.id::text, COALESCE(l.previous_status, ''), l.new_status, COALESCE(l.notes, ''),
		       l.actor_user_id::text, COALESCE(TRIM(CONCAT_WS(' ', u.first_name, u.last_name, u.other_name)), ''),
		       l.created_at
		FROM blood_requisition_status_logs l
		LEFT JOIN users u ON u.id = l.actor_user_id
		WHERE l.blood_requisition_id::text = LOWER($1)
		ORDER BY l.created_at ASC`, strings.TrimSpace(requisitionID))
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var ev blooddomain.BloodRequisitionEvent
		if err := rows.Scan(&ev.ID, &ev.PreviousStatus, &ev.NewStatus, &ev.Notes, &ev.ActorUserID, &ev.ActorName, &ev.CreatedAt); err != nil {
			return out, err
		}
		out.Events = append(out.Events, ev)
	}
	if err := rows.Err(); err != nil {
		return out, err
	}

	var pickup blooddomain.BloodPickupSummary
	err = r.db.QueryRow(ctx, `
		SELECT bta.id::text, bta.status, bta.vehicle_type,
		       COALESCE(a.code, ''), COALESCE(a.plate_number, ''),
		       COALESCE(TRIM(CONCAT_WS(' ', du.first_name, du.last_name, du.other_name)), ''), COALESCE(du.phone, ''),
		       COALESCE(bis.name, ''), COALESCE(df.name, ''), COALESCE(bta.notes, ''),
		       bta.assigned_at, bta.collected_at, bta.delivered_at
		FROM blood_transport_assignments bta
		LEFT JOIN ambulances a ON a.id = bta.ambulance_id
		LEFT JOIN users du ON du.id = bta.assigned_driver_user_id
		LEFT JOIN blood_inventory_sites bis ON bis.id = bta.pickup_site_id
		LEFT JOIN ref_facilities df ON df.id = bta.destination_facility_id
		WHERE bta.blood_requisition_id::text = LOWER($1) AND bta.status <> 'CANCELLED'
		ORDER BY bta.assigned_at DESC
		LIMIT 1`, strings.TrimSpace(requisitionID)).Scan(
		&pickup.ID, &pickup.Status, &pickup.VehicleType,
		&pickup.AmbulanceCode, &pickup.PlateNumber,
		&pickup.DriverName, &pickup.DriverPhone,
		&pickup.PickupSiteName, &pickup.DestinationFacilityName, &pickup.Notes,
		&pickup.AssignedAt, &pickup.CollectedAt, &pickup.DeliveredAt,
	)
	switch {
	case err == nil:
		out.Pickup = &pickup
	case errors.Is(err, pgx.ErrNoRows):
	default:
		return out, err
	}
	return out, nil
}

func (r *Repository) ListRequisitions(ctx context.Context, p platformdb.Pagination) ([]blooddomain.BloodRequisition, int64, error) {
	allowedSorts := map[string]string{
		"created_at":      "br.created_at",
		"status":          "br.status",
		"urgency_level":   "br.urgency_level",
		"units_requested": "br.units_requested",
	}
	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1
	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			COALESCE(br.patient_name,'') ILIKE $%[1]d OR
			COALESCE(br.patient_identifier,'') ILIKE $%[1]d OR
			br.clinical_summary ILIKE $%[1]d OR
			COALESCE(br.diagnosis,'') ILIKE $%[1]d OR
			bg.code ILIKE $%[1]d OR
			bp.code ILIKE $%[1]d OR
			br.status ILIKE $%[1]d
		)`, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}
	for k, v := range p.Filters {
		switch k {
		case "status":
			// Accepts a single status or a comma-separated list.
			where = append(where, fmt.Sprintf(`br.status = ANY($%d)`, argPos))
			args = append(args, splitUpper(v))
			argPos++
		case "urgency_level":
			where = append(where, fmt.Sprintf(`br.urgency_level = $%d`, argPos))
			args = append(args, strings.ToUpper(v))
			argPos++
		case "date_from":
			where = append(where, fmt.Sprintf(`br.created_at >= $%d`, argPos))
			args = append(args, v)
			argPos++
		case "date_to":
			where = append(where, fmt.Sprintf(`br.created_at <= $%d`, argPos))
			args = append(args, v)
			argPos++
		case "requested_by_user_id":
			where = append(where, fmt.Sprintf(`br.requested_by_user_id = $%d`, argPos))
			args = append(args, v)
			argPos++
		}
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")
	var total int64
	countSQL := `SELECT COUNT(1) FROM blood_requisitions br
		JOIN blood_groups bg ON bg.id = br.blood_group_id
		JOIN blood_products bp ON bp.id = br.blood_product_id ` + whereSQL
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	orderBy := platformdb.BuildOrderBy(p, allowedSorts)
	query := fmt.Sprintf(`%s
	%s
	%s
	LIMIT $%d OFFSET $%d`, requisitionSelect, whereSQL, orderBy, argPos, argPos+1)
	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]blooddomain.BloodRequisition, 0)
	for rows.Next() {
		out, err := scanRequisition(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, out)
	}
	return items, total, rows.Err()
}

func (r *Repository) UpdateRequisitionStatus(ctx context.Context, id, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE blood_requisitions SET status=$2, updated_at=now() WHERE id=$1`, id, status)
	return err
}

func (r *Repository) FindBroadcastTargets(ctx context.Context, bloodGroupID, bloodProductID string, unitsRequested int, destLat, destLon *float64, limit int) ([]blooddomain.BloodBroadcastTarget, error) {
	query := `
	SELECT bis.id,
	       bis.name,
	       COALESCE(rd.name, ''),
	       COALESCE(bis.contact_phone, ''),
	       COALESCE(bis.latitude, 0),
	       COALESCE(bis.longitude, 0),
	       CASE
	         WHEN $4::float8 IS NULL OR $5::float8 IS NULL OR bis.location IS NULL THEN 0
	         ELSE ST_Distance(
	            bis.location,
	            ST_SetSRID(ST_MakePoint($5, $4), 4326)::geography
	         ) / 1000.0
	       END AS distance_km,
	       bsu.available_count
	FROM blood_inventory_sites bis
	JOIN blood_stock_units bsu ON bsu.inventory_site_id = bis.id
	LEFT JOIN ref_districts rd ON rd.id = bis.district_id
	WHERE bis.is_active = TRUE
	  AND bsu.blood_group_id = $1
	  AND bsu.blood_product_id = $2
	  AND bsu.available_count >= $3
	ORDER BY distance_km ASC, bsu.available_count DESC
	LIMIT $6`
	rows, err := r.db.Query(ctx, query, bloodGroupID, bloodProductID, unitsRequested, destLat, destLon, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []blooddomain.BloodBroadcastTarget
	for rows.Next() {
		var item blooddomain.BloodBroadcastTarget
		if err := rows.Scan(&item.InventorySiteID, &item.InventorySiteName, &item.DistrictName, &item.ContactPhone, &item.Latitude, &item.Longitude, &item.DistanceKM, &item.AvailableCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *Repository) CreateBroadcasts(ctx context.Context, requisitionID string, message string, targets []blooddomain.BloodBroadcastTarget) error {
	batch := &pgx.Batch{}
	for _, t := range targets {
		batch.Queue(`
			INSERT INTO blood_requisition_broadcasts (
				id, blood_requisition_id, channel, recipient_site_id,
				recipient_phone, message_body, delivery_status, sent_at
			) VALUES (gen_random_uuid(), $1, 'SMS', $2, $3, $4, 'SENT', now())
		`, requisitionID, t.InventorySiteID, t.ContactPhone, message)
	}
	br := r.db.SendBatch(ctx, batch)
	defer br.Close()
	for range targets {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CreateOffer(ctx context.Context, offer blooddomain.BloodRequisitionOffer) (blooddomain.BloodRequisitionOffer, error) {
	query := `
	INSERT INTO blood_requisition_offers (
		id, blood_requisition_id, inventory_site_id, blood_product_id, blood_group_id,
		units_offered, reserved_until, notes, contact_person_name, contact_phone,
		offered_by_user_id, status
	) VALUES (
		$1,$2,$3,$4,$5,
		$6,$7,$8,$9,$10,
		$11,$12
	)
	RETURNING created_at, updated_at`
	err := r.db.QueryRow(ctx, query,
		offer.ID, offer.BloodRequisitionID, offer.InventorySiteID, offer.BloodProductID, offer.BloodGroupID,
		offer.UnitsOffered, offer.ReservedUntil, offer.Notes, offer.ContactPersonName, offer.ContactPhone,
		offer.OfferedByUserID, offer.Status,
	).Scan(&offer.CreatedAt, &offer.UpdatedAt)
	if err != nil {
		return blooddomain.BloodRequisitionOffer{}, err
	}
	return r.getOfferByID(ctx, offer.ID)
}

func (r *Repository) getOfferByID(ctx context.Context, id string) (blooddomain.BloodRequisitionOffer, error) {
	q := `
	SELECT bro.id, bro.blood_requisition_id, bro.inventory_site_id, COALESCE(bis.name,''),
	       bro.blood_product_id, bp.code, bro.blood_group_id, bg.code,
	       bro.units_offered, bro.reserved_until, COALESCE(bro.notes,''), COALESCE(bro.contact_person_name,''), COALESCE(bro.contact_phone,''),
	       bro.offered_by_user_id, bro.status, bro.created_at, bro.updated_at,
	       COALESCE(TRIM(CONCAT_WS(' ', ou.first_name, ou.last_name, ou.other_name)),'')
	FROM blood_requisition_offers bro
	JOIN blood_inventory_sites bis ON bis.id = bro.inventory_site_id
	JOIN blood_products bp ON bp.id = bro.blood_product_id
	JOIN blood_groups bg ON bg.id = bro.blood_group_id
	LEFT JOIN users ou ON ou.id = bro.offered_by_user_id
	WHERE bro.id = $1`
	var out blooddomain.BloodRequisitionOffer
	err := r.db.QueryRow(ctx, q, id).Scan(
		&out.ID, &out.BloodRequisitionID, &out.InventorySiteID, &out.InventorySiteName,
		&out.BloodProductID, &out.BloodProductCode, &out.BloodGroupID, &out.BloodGroupCode,
		&out.UnitsOffered, &out.ReservedUntil, &out.Notes, &out.ContactPersonName, &out.ContactPhone,
		&out.OfferedByUserID, &out.Status, &out.CreatedAt, &out.UpdatedAt,
		&out.OfferedByUserName,
	)
	return out, err
}

func (r *Repository) ListOffers(ctx context.Context, requisitionID string, p platformdb.Pagination) ([]blooddomain.BloodRequisitionOffer, int64, error) {
	allowedSorts := map[string]string{
		"created_at":    "bro.created_at",
		"status":        "bro.status",
		"units_offered": "bro.units_offered",
	}
	where := []string{"bro.blood_requisition_id = $1"}
	args := []any{requisitionID}
	argPos := 2
	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(COALESCE(bis.name,'') ILIKE $%d OR COALESCE(bro.contact_person_name,'') ILIKE $%d OR COALESCE(bro.contact_phone,'') ILIKE $%d)`, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}
	if status, ok := p.Filters["status"]; ok {
		where = append(where, fmt.Sprintf(`bro.status = $%d`, argPos))
		args = append(args, strings.ToUpper(status))
		argPos++
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM blood_requisition_offers bro JOIN blood_inventory_sites bis ON bis.id = bro.inventory_site_id `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	orderBy := platformdb.BuildOrderBy(p, allowedSorts)
	query := fmt.Sprintf(`
	SELECT bro.id, bro.blood_requisition_id, bro.inventory_site_id, COALESCE(bis.name,''),
	       bro.blood_product_id, bp.code, bro.blood_group_id, bg.code,
	       bro.units_offered, bro.reserved_until, COALESCE(bro.notes,''), COALESCE(bro.contact_person_name,''), COALESCE(bro.contact_phone,''),
	       bro.offered_by_user_id, bro.status, bro.created_at, bro.updated_at,
	       COALESCE(TRIM(CONCAT_WS(' ', ou.first_name, ou.last_name, ou.other_name)),'')
	FROM blood_requisition_offers bro
	JOIN blood_inventory_sites bis ON bis.id = bro.inventory_site_id
	JOIN blood_products bp ON bp.id = bro.blood_product_id
	JOIN blood_groups bg ON bg.id = bro.blood_group_id
	LEFT JOIN users ou ON ou.id = bro.offered_by_user_id
	%s
	%s
	LIMIT $%d OFFSET $%d`, whereSQL, orderBy, argPos, argPos+1)
	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := make([]blooddomain.BloodRequisitionOffer, 0)
	for rows.Next() {
		var out blooddomain.BloodRequisitionOffer
		if err := rows.Scan(
			&out.ID, &out.BloodRequisitionID, &out.InventorySiteID, &out.InventorySiteName,
			&out.BloodProductID, &out.BloodProductCode, &out.BloodGroupID, &out.BloodGroupCode,
			&out.UnitsOffered, &out.ReservedUntil, &out.Notes, &out.ContactPersonName, &out.ContactPhone,
			&out.OfferedByUserID, &out.Status, &out.CreatedAt, &out.UpdatedAt,
			&out.OfferedByUserName,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, out)
	}
	return items, total, rows.Err()
}

func (r *Repository) AcceptOffer(ctx context.Context, requisitionID, offerID string) error {
	return platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		// Accept first so an offer from another requisition is rejected
		// before any sibling offers are declined.
		tag, err := tx.Exec(ctx, `UPDATE blood_requisition_offers SET status='ACCEPTED', updated_at=now() WHERE id::text=LOWER($1) AND blood_requisition_id=$2 AND status='OFFERED'`, offerID, requisitionID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			// Distinguish "no such offer" from "offer already accepted/declined".
			var status string
			if err := tx.QueryRow(ctx, `SELECT status FROM blood_requisition_offers WHERE id::text=LOWER($1) AND blood_requisition_id=$2`, offerID, requisitionID).Scan(&status); err != nil {
				return err
			}
			return bloodapp.ErrOfferNotOpen
		}
		// Switching to a newer offer supersedes a previously accepted one.
		if _, err := tx.Exec(ctx, `UPDATE blood_requisition_offers SET status='DECLINED', updated_at=now() WHERE blood_requisition_id=$1 AND id::text <> LOWER($2) AND status IN ('OFFERED','ACCEPTED')`, requisitionID, offerID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE blood_requisitions SET status='MATCHED', updated_at=now() WHERE id=$1`, requisitionID)
		return err
	})
}

func (r *Repository) CreateTransportAssignment(ctx context.Context, in blooddomain.BloodTransportAssignment) (blooddomain.BloodTransportAssignment, error) {
	query := `
	INSERT INTO blood_transport_assignments (
		id, blood_requisition_id, blood_requisition_offer_id, vehicle_type, ambulance_id,
		dispatch_assignment_id, assigned_driver_user_id, assigned_by_user_id,
		pickup_site_id, destination_facility_id, status, notes
	) VALUES (
		$1,$2,$3,$4,$5,
		$6,$7,$8,
		$9,$10,$11,$12
	)
	RETURNING assigned_at`
	err := r.db.QueryRow(ctx, query,
		in.ID, in.BloodRequisitionID, in.BloodRequisitionOfferID, in.VehicleType, in.AmbulanceID,
		in.DispatchAssignmentID, in.AssignedDriverUserID, in.AssignedByUserID,
		in.PickupSiteID, in.DestinationFacilityID, in.Status, in.Notes,
	).Scan(&in.AssignedAt)
	if err != nil {
		return blooddomain.BloodTransportAssignment{}, err
	}
	return in, nil
}

func (r *Repository) MarkTransportCollected(ctx context.Context, assignmentID, requisitionID string) error {
	return r.updateTransport(ctx, `UPDATE blood_transport_assignments SET status='COLLECTED', collected_at=now()
		WHERE id::text=LOWER($1) AND blood_requisition_id=$2`, assignmentID, requisitionID)
}

func (r *Repository) MarkTransportDelivered(ctx context.Context, assignmentID, requisitionID string) error {
	return r.updateTransport(ctx, `UPDATE blood_transport_assignments SET status='DELIVERED', delivered_at=now()
		WHERE id::text=LOWER($1) AND blood_requisition_id=$2`, assignmentID, requisitionID)
}

// updateTransport runs a status update and reports pgx.ErrNoRows when the
// assignment does not exist or belongs to a different requisition.
func (r *Repository) updateTransport(ctx context.Context, q, assignmentID, requisitionID string) error {
	tag, err := r.db.Exec(ctx, q, strings.TrimSpace(assignmentID), requisitionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) CreateStatusLog(ctx context.Context, requisitionID, prevStatus, newStatus string, actorUserID *string, notes string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO blood_requisition_status_logs (id, blood_requisition_id, previous_status, new_status, actor_user_id, notes)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5)
	`, requisitionID, nullIfEmpty(prevStatus), newStatus, actorUserID, notes)
	return err
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// UpdateRequisition applies the supplied fields to a requisition. Group and
// product changes are resolved from their codes.
func (r *Repository) UpdateRequisition(ctx context.Context, id string, req bloodappdto.UpdateBloodRequisitionRequest) (blooddomain.BloodRequisition, error) {
	sets := []string{"updated_at = now()"}
	args := []any{}
	pos := 1

	addString := func(column string, value *string) {
		if value != nil {
			sets = append(sets, fmt.Sprintf("%s = NULLIF($%d,'')", column, pos))
			args = append(args, strings.TrimSpace(*value))
			pos++
		}
	}
	addString("patient_name", req.PatientName)
	addString("patient_identifier", req.PatientIdentifier)
	addString("diagnosis", req.Diagnosis)
	addString("indication", req.Indication)
	addString("parity_summary", req.ParitySummary)
	addString("reporter_phone", req.ReporterPhone)
	addString("requesting_facility_id", req.RequestingFacilityID)
	addString("destination_facility_id", req.DestinationFacilityID)
	if req.ClinicalSummary != nil && strings.TrimSpace(*req.ClinicalSummary) != "" {
		sets = append(sets, fmt.Sprintf("clinical_summary = $%d", pos))
		args = append(args, strings.TrimSpace(*req.ClinicalSummary))
		pos++
	}
	if req.BloodGroupCode != nil {
		sets = append(sets, fmt.Sprintf("blood_group_id = (SELECT id FROM blood_groups WHERE code = $%d)", pos))
		args = append(args, strings.ToUpper(strings.TrimSpace(*req.BloodGroupCode)))
		pos++
	}
	if req.BloodProductCode != nil {
		sets = append(sets, fmt.Sprintf("blood_product_id = (SELECT id FROM blood_products WHERE code = $%d)", pos))
		args = append(args, strings.ToUpper(strings.TrimSpace(*req.BloodProductCode)))
		pos++
	}
	if req.UnitsRequested != nil {
		sets = append(sets, fmt.Sprintf("units_requested = $%d", pos))
		args = append(args, *req.UnitsRequested)
		pos++
	}
	if req.UrgencyLevel != nil {
		sets = append(sets, fmt.Sprintf("urgency_level = $%d", pos))
		args = append(args, strings.ToUpper(strings.TrimSpace(*req.UrgencyLevel)))
		pos++
	}

	args = append(args, id)
	q := fmt.Sprintf("UPDATE blood_requisitions SET %s WHERE id = $%d", strings.Join(sets, ", "), pos)
	if _, err := r.db.Exec(ctx, q, args...); err != nil {
		return blooddomain.BloodRequisition{}, err
	}
	return r.GetRequisitionByID(ctx, id)
}

func (r *Repository) DeleteRequisition(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM blood_requisitions WHERE id = $1`, id)
	return err
}
