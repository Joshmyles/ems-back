package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dispatch/internal/modules/dispatch/application/dto"
	dispatchdomain "dispatch/internal/modules/dispatch/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type triageQuestionDef struct {
	QuestionID   string
	ResponseType string
	TrueScore    *int
	FalseScore   *int
}

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) GetIncidentDispatchContext(ctx context.Context, incidentID string) (dispatchdomain.IncidentDispatchContext, error) {
	var out dispatchdomain.IncidentDispatchContext
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.priority_level_id, COALESCE(rpl.code,''), i.district_id, i.receiving_facility_id, i.latitude, i.longitude, i.verification_status, i.status
		FROM incidents i
		LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
		WHERE i.id=$1`, incidentID,
	).Scan(&out.IncidentID, &out.PriorityLevelID, &out.PriorityCode, &out.DistrictID, &out.FacilityID, &out.Latitude, &out.Longitude, &out.VerificationStatus, &out.Status)
	return out, err
}

func (r *Repository) CountTrueBooleanResponses(ctx context.Context, questionnaireCode string, responses []dispatchdomain.TriageEvaluation) (int, error) {
	boolQuestions := map[string]struct{}{}
	rows, err := r.db.Query(ctx, `
		SELECT tq.code
		FROM triage_questions tq
		JOIN triage_questionnaires tqq ON tqq.id = tq.questionnaire_id
		WHERE tqq.code=$1 AND tq.response_type='BOOLEAN' AND tq.is_active=TRUE`, questionnaireCode)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return 0, err
		}
		boolQuestions[strings.ToUpper(code)] = struct{}{}
	}
	count := 0
	for _, r := range responses {
		if _, ok := boolQuestions[strings.ToUpper(r.QuestionCode)]; !ok {
			continue
		}
		if strings.EqualFold(r.ResponseValue, "true") || strings.EqualFold(r.ResponseValue, "yes") {
			count++
		}
	}
	return count, rows.Err()
}

func (r *Repository) ResolvePriorityCodeByIncident(ctx context.Context, incidentID string) (string, error) {
	var code string
	err := r.db.QueryRow(ctx, `SELECT COALESCE(rpl.code,'') FROM incidents i LEFT JOIN ref_priority_levels rpl ON rpl.id=i.priority_level_id WHERE i.id=$1`, incidentID).Scan(&code)
	return code, err
}

func (r *Repository) SetIncidentPriorityByCode(ctx context.Context, incidentID, priorityCode string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE incidents i
		SET priority_level_id = rpl.id, updated_at=now(), triaged_at=now()
		FROM ref_priority_levels rpl
		WHERE i.id=$1 AND rpl.code=$2`, incidentID, strings.ToUpper(priorityCode))
	return err
}

func (r *Repository) UpdateIncidentStatus(ctx context.Context, incidentID, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE incidents SET status=$2, updated_at=now(), assigned_at=CASE WHEN $2='ASSIGNED' THEN now() ELSE assigned_at END WHERE id=$1`, incidentID, status)
	return err
}

func (r *Repository) CreateIncidentUpdate(ctx context.Context, incidentID, updateType, oldValue, newValue, notes string, actorUserID *string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO incident_updates (id, incident_id, update_type, old_value, new_value, notes, actor_user_id)
		VALUES (gen_random_uuid(), $1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6)
	`, incidentID, updateType, oldValue, newValue, notes, actorUserID)
	return err
}

func (r *Repository) FindDispatchCandidates(ctx context.Context, incidentID string, limit int) ([]dispatchdomain.AmbulanceCandidate, error) {
	query := `
	SELECT DISTINCT ON (ua.current_ambulance_id)
	       ua.current_ambulance_id,
	       CASE WHEN us.user_id IS NOT NULL THEN us.user_id ELSE NULL END AS driver_user_id,
	       us.district_id,
	       us.facility_id,
	       NULL::float8 AS latitude,
	       NULL::float8 AS longitude,
	       ua.availability_status,
	       ua.dispatchable,
	       15 AS eta_minutes
	FROM user_availability ua
	LEFT JOIN user_shifts us ON us.user_id = ua.user_id AND us.status IN ('SCHEDULED','ACTIVE')
	WHERE ua.current_ambulance_id IS NOT NULL
	  AND ua.dispatchable = TRUE
	  AND ua.availability_status IN ('AVAILABLE','STANDBY')
	ORDER BY ua.current_ambulance_id, us.starts_at DESC NULLS LAST
	LIMIT $1`
	rows, err := r.db.Query(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []dispatchdomain.AmbulanceCandidate{}
	for rows.Next() {
		var out dispatchdomain.AmbulanceCandidate
		if err := rows.Scan(&out.AmbulanceID, &out.DriverUserID, &out.DistrictID, &out.FacilityID, &out.Latitude, &out.Longitude, &out.Availability, &out.Dispatchable, &out.ETAMinutes); err != nil {
			return nil, err
		}
		items = append(items, out)
	}
	return items, rows.Err()
}

func (r *Repository) ReplaceRecommendations(ctx context.Context, incidentID string, recs []dispatchdomain.DispatchRecommendation) error {
	return platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM dispatch_recommendations WHERE incident_id=$1`, incidentID); err != nil {
			return err
		}
		for _, rec := range recs {
			_, err := tx.Exec(ctx, `
				INSERT INTO dispatch_recommendations (id, incident_id, ambulance_id, driver_user_id, score, eta_minutes, rule_summary, generated_at, selected)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			`, rec.ID, rec.IncidentID, rec.AmbulanceID, rec.DriverUserID, rec.Score, rec.ETAMinutes, rec.RuleSummary, rec.GeneratedAt, rec.Selected)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ListRecommendations(ctx context.Context, params dto.ListRecommendationsParams) ([]dispatchdomain.DispatchRecommendation, int64, error) {
	p := params.Pagination

	var total int64
	if err := r.db.QueryRow(
		ctx,
		`SELECT COUNT(1) FROM dispatch_recommendations WHERE incident_id = $1`,
		params.IncidentID,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	sortBy := "generated_at"
	sortOrder := "DESC"

	if p.SortBy != "" {
		switch p.SortBy {
		case "generated_at", "score":
			sortBy = p.SortBy
		}
	}

	if p.SortOrder != "" {
		switch strings.ToUpper(p.SortOrder) {
		case "ASC", "DESC":
			sortOrder = strings.ToUpper(p.SortOrder)
		}
	}

	q := fmt.Sprintf(`
		SELECT
			dr.id,
			dr.incident_id,
			dr.ambulance_id,
			dr.driver_user_id,
			dr.score,
			dr.eta_minutes,
			COALESCE(dr.rule_summary, ''),
			dr.generated_at,
			dr.selected,
			COALESCE(a.code, ''),
			COALESCE(a.plate_number, ''),
			COALESCE(TRIM(CONCAT_WS(' ', du.first_name, du.last_name, du.other_name)), '')
		FROM dispatch_recommendations dr
		LEFT JOIN ambulances a ON a.id = dr.ambulance_id
		LEFT JOIN users du ON du.id = dr.driver_user_id
		WHERE dr.incident_id = $1
		ORDER BY dr.%s %s
		LIMIT $2 OFFSET $3
	`, sortBy, sortOrder)

	rows, err := r.db.Query(ctx, q, params.IncidentID, p.PageSize, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []dispatchdomain.DispatchRecommendation{}
	for rows.Next() {
		var out dispatchdomain.DispatchRecommendation
		if err := rows.Scan(
			&out.ID,
			&out.IncidentID,
			&out.AmbulanceID,
			&out.DriverUserID,
			&out.Score,
			&out.ETAMinutes,
			&out.RuleSummary,
			&out.GeneratedAt,
			&out.Selected,
			&out.AmbulanceCode,
			&out.AmbulancePlate,
			&out.DriverName,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, out)
	}

	return items, total, rows.Err()
}

func (r *Repository) CreateAssignment(ctx context.Context, in dispatchdomain.DispatchAssignment) (dispatchdomain.DispatchAssignment, error) {
	// Supersede any lingering open assignment for this incident: a dispatcher
	// re-assigning an incident (e.g. after it was reverted to
	// AWAITING_ASSIGNMENT) expects the stale assignment to be cancelled, not a
	// unique-constraint error from uq_open_dispatch_per_incident.
	if _, err := r.db.Exec(ctx, `
		UPDATE dispatch_assignments
		SET status = 'CANCELLED',
		    cancellation_reason = COALESCE(cancellation_reason, 'Superseded by a new assignment'),
		    updated_at = now()
		WHERE incident_id = $1
		  AND status IN ('PROPOSED','ASSIGNED','ACCEPTED','DEPARTED','ARRIVED_SCENE','PATIENT_LOADED')
	`, in.IncidentID); err != nil {
		return dispatchdomain.DispatchAssignment{}, err
	}

	q := `
	INSERT INTO dispatch_assignments (
		id, incident_id, ambulance_id, assigned_by_user_id, driver_user_id, lead_medic_user_id,
		team_snapshot_json, assignment_mode, ranking_score, eta_minutes, status, assigned_at
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
	RETURNING created_at, updated_at`
	teamJSON := json.RawMessage(in.TeamSnapshotJSON)
	err := r.db.QueryRow(ctx, q, in.ID, in.IncidentID, in.AmbulanceID, in.AssignedByUserID, in.DriverUserID, in.LeadMedicUserID, teamJSON, in.AssignmentMode, in.RankingScore, in.ETAMinutes, in.Status, in.AssignedAt).Scan(&in.CreatedAt, &in.UpdatedAt)
	return in, err
}

func (r *Repository) GetAssignmentByID(ctx context.Context, id string) (dispatchdomain.DispatchAssignmentResponse, error) {
	var out dispatchdomain.DispatchAssignmentResponse

	err := r.db.QueryRow(ctx, `
		SELECT 
			da.id,
			da.incident_id,
			COALESCE(i.incident_number, '') AS incident_number,

			da.ambulance_id,
			COALESCE(a.code, '') AS ambulance_code,
			COALESCE(a.plate_number, '') AS plate_number,
			COALESCE(rac.name, '') AS ambulance_category,

			da.assigned_by_user_id,
			COALESCE(
				TRIM(CONCAT_WS(' ', abu.first_name, abu.last_name, abu.other_name)),
				''
			) AS assigned_by_name,

			da.driver_user_id,
			COALESCE(
				TRIM(CONCAT_WS(' ', du.first_name, du.last_name, du.other_name)),
				''
			) AS driver_name,

			da.lead_medic_user_id,
			COALESCE(
				TRIM(CONCAT_WS(' ', lmu.first_name, lmu.last_name, lmu.other_name)),
				''
			) AS lead_medic_name,

			da.assignment_mode,
			da.ranking_score,
			da.eta_minutes,
			da.status,

			da.assigned_at,
			da.accepted_at,
			da.departed_at,
			da.arrived_scene_at,
			da.patient_loaded_at,
			da.arrived_destination_at,
			da.completed_at,
			da.cancelled_at,

			COALESCE(da.cancellation_reason, '') AS cancellation_reason,
			da.created_at,
			da.updated_at
		FROM dispatch_assignments da
		LEFT JOIN incidents i
			ON i.id = da.incident_id
		LEFT JOIN ambulances a
			ON a.id = da.ambulance_id
		LEFT JOIN ref_ambulance_categories rac
			ON rac.id = a.category_id
		LEFT JOIN users abu
			ON abu.id = da.assigned_by_user_id
		LEFT JOIN users du
			ON du.id = da.driver_user_id
		LEFT JOIN users lmu
			ON lmu.id = da.lead_medic_user_id
		WHERE da.id = $1
	`, id).Scan(
		&out.ID,
		&out.IncidentID,
		&out.IncidentNumber,

		&out.AmbulanceID,
		&out.AmbulanceCode,
		&out.PlateNumber,
		&out.AmbulanceCategory,

		&out.AssignedByUserID,
		&out.AssignedByName,
		&out.DriverUserID,
		&out.DriverName,
		&out.LeadMedicUserID,
		&out.LeadMedicName,

		&out.AssignmentMode,
		&out.RankingScore,
		&out.ETAMinutes,
		&out.Status,

		&out.AssignedAt,
		&out.AcceptedAt,
		&out.DepartedAt,
		&out.ArrivedSceneAt,
		&out.PatientLoadedAt,
		&out.ArrivedDestinationAt,
		&out.CompletedAt,
		&out.CancelledAt,

		&out.CancellationReason,
		&out.CreatedAt,
		&out.UpdatedAt,
	)

	return out, err
}

func (r *Repository) UpdateAssignmentStatus(ctx context.Context, id, status, cancellationReason string) (dispatchdomain.DispatchAssignmentResponse, error) {
	now := time.Now().UTC()

	q := `
		UPDATE dispatch_assignments
		SET status = $2,
			accepted_at = CASE 
				WHEN $2 = 'ACCEPTED' AND accepted_at IS NULL THEN $3 
				ELSE accepted_at 
			END,
			departed_at = CASE 
				WHEN $2 = 'DEPARTED' AND departed_at IS NULL THEN $3 
				ELSE departed_at 
			END,
			arrived_scene_at = CASE 
				WHEN $2 = 'ARRIVED_SCENE' AND arrived_scene_at IS NULL THEN $3 
				ELSE arrived_scene_at 
			END,
			patient_loaded_at = CASE 
				WHEN $2 = 'PATIENT_LOADED' AND patient_loaded_at IS NULL THEN $3 
				ELSE patient_loaded_at 
			END,
			arrived_destination_at = CASE 
				WHEN $2 = 'ARRIVED_DESTINATION' AND arrived_destination_at IS NULL THEN $3 
				ELSE arrived_destination_at 
			END,
			completed_at = CASE 
				WHEN $2 = 'COMPLETED' AND completed_at IS NULL THEN $3 
				ELSE completed_at 
			END,
			cancelled_at = CASE 
				WHEN $2 = 'CANCELLED' AND cancelled_at IS NULL THEN $3 
				ELSE cancelled_at 
			END,
			cancellation_reason = CASE 
				WHEN $2 = 'CANCELLED' THEN $4
				ELSE cancellation_reason
			END,
			updated_at = $3
		WHERE id = $1
	`
	ct, err := r.db.Exec(ctx, q, id, strings.ToUpper(status), now, cancellationReason)
	if err != nil {
		return dispatchdomain.DispatchAssignmentResponse{}, err
	}
	if ct.RowsAffected() == 0 {
		return dispatchdomain.DispatchAssignmentResponse{}, pgx.ErrNoRows
	}

	return r.GetAssignmentByID(ctx, id)
}

func (r *Repository) ListAssignments(ctx context.Context, params dto.ListAssignmentsParams) ([]dispatchdomain.DispatchAssignmentResponse, int64, error) {
	p := params.Pagination
	where := []string{"1=1"}
	args := []any{}
	pos := 1

	if params.IncidentID != nil && *params.IncidentID != "" {
		where = append(where, fmt.Sprintf("da.incident_id = $%d", pos))
		args = append(args, *params.IncidentID)
		pos++
	}

	if params.AmbulanceID != nil && *params.AmbulanceID != "" {
		where = append(where, fmt.Sprintf("da.ambulance_id = $%d", pos))
		args = append(args, *params.AmbulanceID)
		pos++
	}

	if params.Status != nil && *params.Status != "" {
		where = append(where, fmt.Sprintf("da.status = $%d", pos))
		args = append(args, strings.ToUpper(*params.Status))
		pos++
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	countQuery := `
		SELECT COUNT(1)
		FROM dispatch_assignments da
		` + whereSQL

	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, map[string]string{
		"created_at":  "da.created_at",
		"status":      "da.status",
		"assigned_at": "da.assigned_at",
	})

	q := fmt.Sprintf(`
		SELECT 
			da.id,
			da.incident_id,
			COALESCE(i.incident_number, '') AS incident_number,

			da.ambulance_id,
			COALESCE(a.code, '') AS ambulance_code,
			COALESCE(a.plate_number, '') AS plate_number,
			COALESCE(rac.name, '') AS ambulance_category,

			da.assigned_by_user_id,
			COALESCE(
				TRIM(CONCAT_WS(' ', abu.first_name, abu.last_name, abu.other_name)),
				''
			) AS assigned_by_name,

			da.driver_user_id,
			COALESCE(
				TRIM(CONCAT_WS(' ', du.first_name, du.last_name, du.other_name)),
				''
			) AS driver_name,

			da.lead_medic_user_id,
			COALESCE(
				TRIM(CONCAT_WS(' ', lmu.first_name, lmu.last_name, lmu.other_name)),
				''
			) AS lead_medic_name,

			da.assignment_mode,
			da.ranking_score,
			da.eta_minutes,
			da.status,

			da.assigned_at,
			da.accepted_at,
			da.departed_at,
			da.arrived_scene_at,
			da.patient_loaded_at,
			da.arrived_destination_at,
			da.completed_at,
			da.cancelled_at,

			COALESCE(da.cancellation_reason, '') AS cancellation_reason,
			da.created_at,
			da.updated_at
		FROM dispatch_assignments da
		LEFT JOIN incidents i
			ON i.id = da.incident_id
		LEFT JOIN ambulances a
			ON a.id = da.ambulance_id
		LEFT JOIN ref_ambulance_categories rac
			ON rac.id = a.category_id
		LEFT JOIN users abu
			ON abu.id = da.assigned_by_user_id
		LEFT JOIN users du
			ON du.id = da.driver_user_id
		LEFT JOIN users lmu
			ON lmu.id = da.lead_medic_user_id
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, pos, pos+1)

	rows, err := r.db.Query(ctx, q, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []dispatchdomain.DispatchAssignmentResponse{}
	for rows.Next() {
		var out dispatchdomain.DispatchAssignmentResponse
		if err := rows.Scan(
			&out.ID,
			&out.IncidentID,
			&out.IncidentNumber,

			&out.AmbulanceID,
			&out.AmbulanceCode,
			&out.PlateNumber,
			&out.AmbulanceCategory,

			&out.AssignedByUserID,
			&out.AssignedByName,
			&out.DriverUserID,
			&out.DriverName,
			&out.LeadMedicUserID,
			&out.LeadMedicName,

			&out.AssignmentMode,
			&out.RankingScore,
			&out.ETAMinutes,
			&out.Status,

			&out.AssignedAt,
			&out.AcceptedAt,
			&out.DepartedAt,
			&out.ArrivedSceneAt,
			&out.PatientLoadedAt,
			&out.ArrivedDestinationAt,
			&out.CompletedAt,
			&out.CancelledAt,

			&out.CancellationReason,
			&out.CreatedAt,
			&out.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, out)
	}

	return items, total, rows.Err()
}

func (r *Repository) MarkRecommendationSelected(ctx context.Context, incidentID, ambulanceID string) error {
	_, err := r.db.Exec(ctx, `UPDATE dispatch_recommendations SET selected = CASE WHEN ambulance_id=$2 THEN TRUE ELSE FALSE END WHERE incident_id=$1`, incidentID, ambulanceID)
	return err
}

func (r *Repository) SetUserAvailabilityBusy(ctx context.Context, incidentID, assignmentID string, ambulanceID, driverUserID, medicUserID *string) error {
	return platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		for _, userID := range []*string{driverUserID, medicUserID} {
			if userID == nil || *userID == "" {
				continue
			}
			_, err := tx.Exec(ctx, `
				INSERT INTO user_availability (id, user_id, availability_status, dispatchable, current_incident_id, current_dispatch_assignment_id, current_ambulance_id, source, updated_at)
				VALUES (gen_random_uuid(), $1, 'BUSY', FALSE, $2, $3, $4, 'SYSTEM', now())
				ON CONFLICT (user_id) DO UPDATE SET
					availability_status='BUSY', dispatchable=FALSE, current_incident_id=$2, current_dispatch_assignment_id=$3, current_ambulance_id=$4, source='SYSTEM', updated_at=now()
			`, *userID, incidentID, assignmentID, ambulanceID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *Repository) ResolveQuestionnaireIDByCode(ctx context.Context, questionnaireCode string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id
		FROM triage_questionnaires
		WHERE code = $1 AND is_active = TRUE
	`, strings.ToUpper(questionnaireCode)).Scan(&id)
	return id, err
}

func (r *Repository) GetQuestionDefinitions(ctx context.Context, questionnaireCode string) (map[string]dispatchdomain.QuestionDefinition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tq.id, tq.code, tq.response_type,
		       MAX(CASE WHEN tqo.option_value = 'true' THEN tqo.score END) AS true_score,
		       MAX(CASE WHEN tqo.option_value = 'false' THEN tqo.score END) AS false_score
		FROM triage_questions tq
		JOIN triage_questionnaires tqq ON tqq.id = tq.questionnaire_id
		LEFT JOIN triage_question_options tqo ON tqo.question_id = tq.id
		WHERE tqq.code = $1 AND tq.is_active = TRUE
		GROUP BY tq.id, tq.code, tq.response_type
	`, strings.ToUpper(questionnaireCode))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]dispatchdomain.QuestionDefinition{}
	for rows.Next() {
		var qid, code, respType string
		var trueScore, falseScore *int
		if err := rows.Scan(&qid, &code, &respType, &trueScore, &falseScore); err != nil {
			return nil, err
		}
		out[strings.ToUpper(code)] = dispatchdomain.QuestionDefinition{
			QuestionID:   qid,
			ResponseType: respType,
			TrueScore:    trueScore,
			FalseScore:   falseScore,
		}
	}
	return out, rows.Err()
}

func (r *Repository) ResolvePriorityLevelIDByCode(ctx context.Context, priorityCode string) (*string, error) {
	var id string
	err := r.db.QueryRow(ctx, `
		SELECT id FROM ref_priority_levels WHERE code = $1
	`, strings.ToUpper(priorityCode)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *Repository) CreatePersistedTriageSession(ctx context.Context, session dispatchdomain.PersistedTriageSession) (dispatchdomain.PersistedTriageSession, error) {
	err := platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO incident_triage_sessions (
				id, incident_id, questionnaire_id, triage_mode, total_score,
				boolean_true_count, auto_dispatch_eligible, derived_priority_level_id,
				notes, triaged_by_user_id, triaged_at, created_at, updated_at
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now())
		`,
			session.ID, session.IncidentID, session.QuestionnaireID, session.TriageMode,
			session.TotalScore, session.BooleanTrueCount, session.AutoDispatchEligible,
			session.DerivedPriorityLevelID, session.Notes, session.TriagedByUserID, session.TriagedAt,
		)
		if err != nil {
			return err
		}

		for _, resp := range session.Responses {
			_, err := tx.Exec(ctx, `
				INSERT INTO incident_triage_responses (
					id, triage_session_id, incident_id, question_id, question_code, response_type,
					response_value_text, response_value_bool, response_value_int,
					selected_option_id, selected_option_code, score_awarded, created_at, updated_at
				)
				VALUES (gen_random_uuid(), $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now())
			`,
				session.ID, session.IncidentID, resp.QuestionID, resp.QuestionCode, resp.ResponseType,
				resp.ResponseValueText, resp.ResponseValueBool, resp.ResponseValueInt,
				resp.SelectedOptionID, resp.SelectedOptionCode, resp.ScoreAwarded,
			)
			if err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return dispatchdomain.PersistedTriageSession{}, err
	}
	return r.GetLatestTriageSession(ctx, session.IncidentID)
}

func (r *Repository) GetLatestTriageSession(ctx context.Context, incidentID string) (dispatchdomain.PersistedTriageSession, error) {
	var out dispatchdomain.PersistedTriageSession
	err := r.db.QueryRow(ctx, `
		SELECT its.id, its.incident_id, its.questionnaire_id, its.triage_mode, its.total_score,
		       its.boolean_true_count, its.auto_dispatch_eligible, its.derived_priority_level_id,
		       COALESCE(rpl.code,''), COALESCE(its.notes,''), its.triaged_by_user_id, its.triaged_at,
		       its.created_at, its.updated_at
		FROM incident_triage_sessions its
		LEFT JOIN ref_priority_levels rpl ON rpl.id = its.derived_priority_level_id
		WHERE its.incident_id = $1
		ORDER BY its.triaged_at DESC, its.created_at DESC
		LIMIT 1
	`, incidentID).Scan(
		&out.ID, &out.IncidentID, &out.QuestionnaireID, &out.TriageMode, &out.TotalScore,
		&out.BooleanTrueCount, &out.AutoDispatchEligible, &out.DerivedPriorityLevelID,
		&out.DerivedPriorityCode, &out.Notes, &out.TriagedByUserID, &out.TriagedAt,
		&out.CreatedAt, &out.UpdatedAt,
	)
	if err != nil {
		return out, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT question_id, question_code, response_type, response_value_text, response_value_bool,
		       response_value_int, selected_option_id, selected_option_code, score_awarded
		FROM incident_triage_responses
		WHERE triage_session_id = $1
		ORDER BY created_at ASC
	`, out.ID)
	if err != nil {
		return out, err
	}
	defer rows.Close()

	out.Responses = []dispatchdomain.PersistedTriageResponse{}
	for rows.Next() {
		var resp dispatchdomain.PersistedTriageResponse
		if err := rows.Scan(
			&resp.QuestionID, &resp.QuestionCode, &resp.ResponseType, &resp.ResponseValueText,
			&resp.ResponseValueBool, &resp.ResponseValueInt, &resp.SelectedOptionID,
			&resp.SelectedOptionCode, &resp.ScoreAwarded,
		); err != nil {
			return out, err
		}
		out.Responses = append(out.Responses, resp)
	}
	return out, rows.Err()
}

func (r *Repository) SetIncidentTriageSummary(ctx context.Context, incidentID string, triagedByUserID *string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE incidents
		SET triaged_by_user_id = $2,
		    triaged_at = now(),
		    updated_at = now()
		WHERE id = $1
	`, incidentID, triagedByUserID)
	return err
}
