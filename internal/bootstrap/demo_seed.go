package bootstrap

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeedDemoData inserts a small, linked set of demo records for:
// fleet (ambulances), fuel logs, incidents, dispatch assignments, trips,
// notifications, and blood (stock + requisitions).
//
// It is designed to be idempotent – running it multiple times will not
// create duplicates, and it assumes migrations + facility seeding have
// already run.
func SeedDemoData(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	now := time.Now().UTC()

	// ------------------------------------------------------------------
	// Core references: admin user, district, facility
	// ------------------------------------------------------------------
	var adminUserID string
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = 'admin' LIMIT 1`).Scan(&adminUserID); err != nil {
		return fmt.Errorf("admin user not found, run base seed first: %w", err)
	}

	var districtID string
	if err := tx.QueryRow(ctx, `SELECT id FROM ref_districts LIMIT 1`).Scan(&districtID); err != nil {
		return fmt.Errorf("no districts found; run facility seed first: %w", err)
	}

	var facilityID string
	if err := tx.QueryRow(ctx, `SELECT id FROM ref_facilities LIMIT 1`).Scan(&facilityID); err != nil {
		return fmt.Errorf("no facilities found; run facility seed first: %w", err)
	}

	// ------------------------------------------------------------------
	// Fleet: ensure at least one ambulance category and a demo ambulance
	// ------------------------------------------------------------------
	var categoryID string
	err = tx.QueryRow(ctx, `SELECT id FROM ref_ambulance_categories LIMIT 1`).Scan(&categoryID)
	if err != nil {
		// Create a simple default category if none exist
		if err := tx.QueryRow(
			ctx,
			`INSERT INTO ref_ambulance_categories (code, name, description)
			 VALUES ($1,$2,$3)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			"GEN", "General Ambulance", "Demo general ambulance category",
		).Scan(&categoryID); err != nil {
			return fmt.Errorf("failed to ensure ambulance category: %w", err)
		}
	}

	var ambulanceID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO ambulances (
			code, plate_number, vin, make, model, year_of_manufacture,
			category_id, ownership_type, station_facility_id, district_id,
			status, dispatch_readiness, is_active, created_at, updated_at
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,$10,
			'AVAILABLE','DISPATCHABLE',true,$11,$11
		)
		ON CONFLICT (code) DO UPDATE
		    SET plate_number = EXCLUDED.plate_number,
		        station_facility_id = EXCLUDED.station_facility_id,
		        district_id = EXCLUDED.district_id,
		        updated_at = EXCLUDED.updated_at
		RETURNING id`,
		"AMB-DEMO-001",
		"UAA 000D",
		"VIN-DEMO-001",
		"Toyota",
		"Hiace",
		2015,
		categoryID,
		"GOVERNMENT",
		facilityID,
		districtID,
		now,
	).Scan(&ambulanceID); err != nil {
		return fmt.Errorf("failed to seed demo ambulance: %w", err)
	}

	// ------------------------------------------------------------------
	// Incidents: pick or create a basic incident type / priority / severity
	// ------------------------------------------------------------------
	var incidentTypeID string
	if err := tx.QueryRow(ctx, `SELECT id FROM ref_incident_types LIMIT 1`).Scan(&incidentTypeID); err != nil {
		if err := tx.QueryRow(
			ctx,
			`INSERT INTO ref_incident_types (code, name)
			 VALUES ($1,$2)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			"GENERIC",
			"Generic Emergency",
		).Scan(&incidentTypeID); err != nil {
			return fmt.Errorf("failed to ensure incident type: %w", err)
		}
	}

	var priorityID string
	if err := tx.QueryRow(ctx, `SELECT id FROM ref_priority_levels LIMIT 1`).Scan(&priorityID); err != nil {
		if err := tx.QueryRow(
			ctx,
			`INSERT INTO ref_priority_levels (code, name)
			 VALUES ($1,$2)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			"EMERGENCY",
			"Emergency",
		).Scan(&priorityID); err != nil {
			return fmt.Errorf("failed to ensure priority level: %w", err)
		}
	}

	var severityID string
	if err := tx.QueryRow(ctx, `SELECT id FROM ref_severity_levels LIMIT 1`).Scan(&severityID); err != nil {
		if err := tx.QueryRow(
			ctx,
			`INSERT INTO ref_severity_levels (code, name)
			 VALUES ($1,$2)
			 ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
			 RETURNING id`,
			"SEVERE",
			"Severe",
		).Scan(&severityID); err != nil {
			return fmt.Errorf("failed to ensure severity level: %w", err)
		}
	}

	var incidentID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO incidents (
			incident_number, source_channel, caller_name, caller_phone,
			patient_name, patient_sex, incident_type_id, severity_level_id,
			priority_level_id, summary, description, district_id, referring_facility_id,
			status, verification_status, reported_at, created_by_user_id, created_at, updated_at
		) VALUES (
			$1,'CALL',$2,$3,
			$4,'MALE',$5,$6,
			$7,$8,$9,$10,$11,
			'ASSIGNED','VERIFIED',$12,$13,$12,$12
		)
		ON CONFLICT (incident_number) DO UPDATE
		    SET summary = EXCLUDED.summary,
		        description = EXCLUDED.description,
		        status = EXCLUDED.status,
		        verification_status = EXCLUDED.verification_status,
		        updated_at = EXCLUDED.updated_at
		RETURNING id`,
		"INC-DEMO-001",
		"John Doe",
		"+256780000001",
		"Demo Patient",
		incidentTypeID,
		severityID,
		priorityID,
		"Demo incident for seeding",
		"Demo emergency incident used for linked demo data",
		districtID,
		facilityID,
		now,
		adminUserID,
	).Scan(&incidentID); err != nil {
		return fmt.Errorf("failed to seed demo incident: %w", err)
	}

	// ------------------------------------------------------------------
	// Dispatch assignment (dispatch module)
	// ------------------------------------------------------------------
	var dispatchID string
	// First try to reuse an existing assignment for this incident (idempotent)
	err = tx.QueryRow(ctx, `SELECT id FROM dispatch_assignments WHERE incident_id = $1 ORDER BY created_at LIMIT 1`, incidentID).Scan(&dispatchID)
	if err != nil {
		if err != pgx.ErrNoRows {
			return fmt.Errorf("failed to lookup existing dispatch assignment: %w", err)
		}
		// No existing assignment; create a new one
		if err := tx.QueryRow(
			ctx,
			`INSERT INTO dispatch_assignments (
				incident_id, ambulance_id, assigned_by_user_id,
				driver_user_id, assignment_mode, status, assigned_at, created_at, updated_at
			) VALUES (
				$1,$2,$3,
				$4,'MANUAL','ASSIGNED',$5,$5,$5
			)
			RETURNING id`,
			incidentID,
			ambulanceID,
			adminUserID,
			adminUserID,
			now,
		).Scan(&dispatchID); err != nil {
			return fmt.Errorf("failed to seed demo dispatch assignment: %w", err)
		}
	}

	// ------------------------------------------------------------------
	// Trip + events
	// ------------------------------------------------------------------
	var tripID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO trips (
			dispatch_assignment_id, incident_id, ambulance_id,
			origin_lat, origin_lon,
			scene_lat, scene_lon,
			destination_facility_id, destination_lat, destination_lon,
			odometer_start, odometer_end,
			started_at, ended_at, outcome, notes,
			created_at, updated_at
		) VALUES (
			$1,$2,$3,
			0.0,0.0,
			0.0,0.0,
			$4,0.0,0.0,
			10000.0,10025.0,
			$5,$6,'COMPLETED','Demo trip for seeding',
			$5,$6
		)
		ON CONFLICT (dispatch_assignment_id) DO UPDATE
		    SET outcome = EXCLUDED.outcome,
		        notes = EXCLUDED.notes,
		        odometer_start = EXCLUDED.odometer_start,
		        odometer_end = EXCLUDED.odometer_end,
		        started_at = EXCLUDED.started_at,
		        ended_at = EXCLUDED.ended_at,
		        updated_at = EXCLUDED.updated_at
		RETURNING id`,
		dispatchID,
		incidentID,
		ambulanceID,
		facilityID,
		now.Add(-30*time.Minute),
		now,
	).Scan(&tripID); err != nil {
		return fmt.Errorf("failed to seed demo trip: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO trip_events (
			trip_id, event_type, event_time, actor_user_id, notes
		) VALUES
			($1,'ASSIGNED',$2,$7,'Trip assigned'),
			($1,'DEPARTED',$3,$7,'Ambulance departed'),
			($1,'ARRIVED_SCENE',$4,$7,'Arrived at scene'),
			($1,'PATIENT_LOADED',$5,$7,'Patient loaded'),
			($1,'ARRIVED_DESTINATION',$6,$7,'Arrived at destination'),
			($1,'COMPLETED',$6,$7,'Trip completed')
		ON CONFLICT DO NOTHING`,
		tripID,
		now.Add(-25*time.Minute),
		now.Add(-20*time.Minute),
		now.Add(-15*time.Minute),
		now.Add(-10*time.Minute),
		now,
		adminUserID,
	); err != nil {
		return fmt.Errorf("failed to seed trip events: %w", err)
	}

	// ------------------------------------------------------------------
	// Fuel logs
	// ------------------------------------------------------------------
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO fuel_logs (
			ambulance_id, fuel_type, liters, cost, odometer_km,
			station_name, filled_at, filled_by, notes, public_token
		) VALUES
			($1,'Diesel',40.0,220000,10000,'Demo Station',$2,$3,'Initial full tank',$5),
			($1,'Diesel',15.0,90000,10025,'Demo Station',$4,$3,'Top up after trip',$6)
		ON CONFLICT DO NOTHING`,
		ambulanceID,
		now.Add(-2*time.Hour),
		adminUserID,
		now.Add(-45*time.Minute),
		uuid.NewString(),
		uuid.NewString(),
	); err != nil {
		return fmt.Errorf("failed to seed fuel logs: %w", err)
	}

	// ------------------------------------------------------------------
	// Notifications
	// ------------------------------------------------------------------
	if _, err := tx.Exec(
		ctx,
		`INSERT INTO notifications (
			id, type, recipient_user_id, title, body, channel,
			status, attempts, created_at
		) VALUES
			($1,'TRIP_COMPLETED',$2,'Trip completed','Trip INC-DEMO-001 has been completed','IN_APP','DELIVERED',1,$3),
			($4,'BLOOD_REQUISITION_OPEN',$2,'Blood requisition opened','A new blood requisition has been created','IN_APP','PENDING',0,$3)
		ON CONFLICT DO NOTHING`,
		uuid.NewString(),
		adminUserID,
		now,
		uuid.NewString(),
	); err != nil {
		return fmt.Errorf("failed to seed notifications: %w", err)
	}

	// ------------------------------------------------------------------
	// Blood: stock units and a requisition linked to the same incident
	// ------------------------------------------------------------------
	var bloodGroupID string
	if err := tx.QueryRow(ctx, `SELECT id FROM blood_groups WHERE code = 'O+'`).Scan(&bloodGroupID); err != nil {
		return fmt.Errorf("blood group O+ not found (check blood reference migration): %w", err)
	}

	var bloodProductID string
	if err := tx.QueryRow(ctx, `SELECT id FROM blood_products WHERE code = 'WB'`).Scan(&bloodProductID); err != nil {
		return fmt.Errorf("blood product WB not found (check blood reference migration): %w", err)
	}

	// Ensure an inventory site at the demo facility
	var inventorySiteID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO blood_inventory_sites (
			site_type, facility_id, code, name, district_id,
			can_issue_emergency_blood, is_active, created_at, updated_at
		) VALUES (
			'FACILITY',$1,$2,$3,$4,
			true,true,$5,$5
		)
		ON CONFLICT (code) DO UPDATE
		    SET facility_id = EXCLUDED.facility_id,
		        district_id = EXCLUDED.district_id,
		        is_active = EXCLUDED.is_active,
		        updated_at = EXCLUDED.updated_at
		RETURNING id`,
		facilityID,
		"INV-DEMO-001",
		"Demo Facility Blood Inventory",
		districtID,
		now,
	).Scan(&inventorySiteID); err != nil {
		return fmt.Errorf("failed to ensure blood inventory site: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`INSERT INTO blood_stock_units (
			inventory_site_id, blood_product_id, blood_group_id,
			unit_count, reserved_count, updated_by
		) VALUES (
			$1,$2,$3,
			20,5,$4
		)
		ON CONFLICT (inventory_site_id, blood_product_id, blood_group_id) DO UPDATE
		    SET unit_count = EXCLUDED.unit_count,
		        reserved_count = EXCLUDED.reserved_count,
		        last_updated_at = now(),
		        updated_by = EXCLUDED.updated_by`,
		inventorySiteID,
		bloodProductID,
		bloodGroupID,
		adminUserID,
	); err != nil {
		return fmt.Errorf("failed to seed blood stock units: %w", err)
	}

	var requisitionID string
	if err := tx.QueryRow(
		ctx,
		`INSERT INTO blood_requisitions (
			incident_id, requesting_facility_id,
			patient_name, clinical_summary, diagnosis,
			blood_group_id, blood_product_id, units_requested,
			urgency_level, status, reporter_phone,
			destination_facility_id, requested_by_user_id,
			created_at, updated_at
		) VALUES (
			$1,$2,
			$3,$4,$5,
			$6,$7,2,
			'EMERGENCY','OPEN',$8,
			$2,$9,
			$10,$10
		)
		ON CONFLICT DO NOTHING
		RETURNING id`,
		incidentID,
		facilityID,
		"Demo Patient",
		"Severe haemorrhage, requires whole blood",
		"Post-partum haemorrhage",
		bloodGroupID,
		bloodProductID,
		"+256780000002",
		adminUserID,
		now,
	).Scan(&requisitionID); err != nil && err.Error() != "no rows in result set" {
		return fmt.Errorf("failed to seed blood requisition: %w", err)
	}

	// Optionally create a transport assignment if we created a new requisition
	if requisitionID != "" {
		if _, err := tx.Exec(
			ctx,
			`INSERT INTO blood_transport_assignments (
				blood_requisition_id, vehicle_type, ambulance_id,
				dispatch_assignment_id, assigned_driver_user_id,
				assigned_by_user_id, destination_facility_id,
				status, assigned_at
			) VALUES (
				$1,'AMBULANCE',$2,
				$3,$4,
				$4,$5,
				'ASSIGNED',$6
			)
			ON CONFLICT DO NOTHING`,
			requisitionID,
			ambulanceID,
			dispatchID,
			adminUserID,
			facilityID,
			now,
		); err != nil {
			return fmt.Errorf("failed to seed blood transport assignment: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	fmt.Println("demo data seeding completed")
	return nil
}
