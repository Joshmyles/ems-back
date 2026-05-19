package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	incidentapp "dispatch/internal/modules/incidents/application"
	incidentdomain "dispatch/internal/modules/incidents/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

func (r *Repository) NextIncidentNumber(ctx context.Context) (string, error) {
	var count int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM incidents`).Scan(&count); err != nil {
		return "", err
	}
	return fmt.Sprintf("INC-%s-%06d", time.Now().UTC().Format("20060102"), count+1), nil
}

func nilIfBlank(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func trimmedValue(s *string) string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

func (r *Repository) CreateIncident(ctx context.Context, in incidentdomain.Incident) (incidentdomain.Incident, error) {
	// normalize optional UUID fields
	in.FacilityID = nilIfBlank(in.FacilityID)
	in.DistrictID = nilIfBlank(in.DistrictID)
	in.SeverityLevelID = nilIfBlank(in.SeverityLevelID)
	in.PriorityLevelID = nilIfBlank(in.PriorityLevelID)
	in.CreatedByUserID = nilIfBlank(in.CreatedByUserID)

	q := `
	INSERT INTO incidents (
		id, incident_number, source_channel, caller_name, caller_phone, patient_name, patient_phone,
		patient_age_group, patient_sex, incident_type_id, severity_level_id, priority_level_id,
		summary, description, district_id, facility_id, village, parish, subcounty, landmark,
		latitude, longitude, verification_status, status, reported_at, created_by_user_id, created_at, updated_at
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,
		$8,$9,$10,$11,$12,
		$13,$14,$15,$16,$17,$18,$19,$20,
		$21,$22,$23,$24,$25,$26,now(),now()
	)
	RETURNING triaged_by_user_id, triaged_at, assigned_at, closed_at, created_at, updated_at`

	err := r.db.QueryRow(ctx, q,
		in.ID,
		in.IncidentNumber,
		in.SourceChannel,
		in.CallerName,
		in.CallerPhone,
		in.PatientName,
		in.PatientPhone,
		in.PatientAgeGroup,
		in.PatientSex,
		in.IncidentTypeID,
		in.SeverityLevelID,
		in.PriorityLevelID,
		in.Summary,
		in.Description,
		in.DistrictID,
		in.FacilityID,
		in.Village,
		in.Parish,
		in.Subcounty,
		in.Landmark,
		in.Latitude,
		in.Longitude,
		in.VerificationStatus,
		in.Status,
		in.ReportedAt,
		in.CreatedByUserID,
	).Scan(
		&in.TriagedByUserID,
		&in.TriagedAt,
		&in.AssignedAt,
		&in.ClosedAt,
		&in.CreatedAt,
		&in.UpdatedAt,
	)
	if err != nil {
		return incidentdomain.Incident{}, err
	}

	return r.GetIncidentByID(ctx, in.ID)
}

func (r *Repository) GetIncidentByID(ctx context.Context, id string) (incidentdomain.Incident, error) {
	var out incidentdomain.Incident
	err := r.db.QueryRow(ctx, `
		SELECT i.id, i.incident_number, i.source_channel, COALESCE(i.caller_name,''), COALESCE(i.caller_phone,''),
		COALESCE(i.patient_name,''), COALESCE(i.patient_phone,''), COALESCE(i.patient_age_group,''), COALESCE(i.patient_sex,''),
		i.incident_type_id, i.severity_level_id, i.priority_level_id, COALESCE(rpl.code,''), COALESCE(i.summary,''), COALESCE(i.description,''),
		i.district_id, i.facility_id, COALESCE(i.village,''), COALESCE(i.parish,''), COALESCE(i.subcounty,''), COALESCE(i.landmark,''),
		i.latitude, i.longitude, i.verification_status, i.status, i.reported_at, i.created_by_user_id, i.triaged_by_user_id,
		i.triaged_at, i.assigned_at, i.closed_at, i.created_at, i.updated_at
		FROM incidents i
		LEFT JOIN ref_priority_levels rpl ON rpl.id = i.priority_level_id
		WHERE i.id=$1`, id,
	).Scan(&out.ID, &out.IncidentNumber, &out.SourceChannel, &out.CallerName, &out.CallerPhone,
		&out.PatientName, &out.PatientPhone, &out.PatientAgeGroup, &out.PatientSex,
		&out.IncidentTypeID, &out.SeverityLevelID, &out.PriorityLevelID, &out.PriorityCode, &out.Summary, &out.Description,
		&out.DistrictID, &out.FacilityID, &out.Village, &out.Parish, &out.Subcounty, &out.Landmark,
		&out.Latitude, &out.Longitude, &out.VerificationStatus, &out.Status, &out.ReportedAt, &out.CreatedByUserID, &out.TriagedByUserID,
		&out.TriagedAt, &out.AssignedAt, &out.ClosedAt, &out.CreatedAt, &out.UpdatedAt)
	return out, err
}

func (r *Repository) ListIncidents(ctx context.Context, params incidentapp.ListIncidentsParams) ([]incidentdomain.Incident, int64, error) {
	p := params.Pagination
	where := []string{"1=1"}
	args := []any{}
	pos := 1
	if params.Status != nil && *params.Status != "" {
		where = append(where, fmt.Sprintf("i.status=$%d", pos))
		args = append(args, strings.ToUpper(*params.Status))
		pos++
	}
	if params.DistrictID != nil && *params.DistrictID != "" {
		where = append(where, fmt.Sprintf("i.district_id=$%d", pos))
		args = append(args, *params.DistrictID)
		pos++
	}
	if params.FacilityID != nil && *params.FacilityID != "" {
		where = append(where, fmt.Sprintf("i.facility_id=$%d", pos))
		args = append(args, *params.FacilityID)
		pos++
	}
	if params.PriorityID != nil && *params.PriorityID != "" {
		where = append(where, fmt.Sprintf("i.priority_level_id=$%d", pos))
		args = append(args, *params.PriorityID)
		pos++
	}
	if p.Search != "" {
		where = append(where, fmt.Sprintf("(i.incident_number ILIKE $%d OR COALESCE(i.summary,'') ILIKE $%d OR COALESCE(i.patient_name,'') ILIKE $%d)", pos, pos, pos))
		args = append(args, "%"+p.Search+"%")
		pos++
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")
	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM incidents i `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	q := fmt.Sprintf(`SELECT i.id, i.incident_number, i.source_channel, COALESCE(i.caller_name,''), COALESCE(i.caller_phone,''), COALESCE(i.patient_name,''), COALESCE(i.patient_phone,''), COALESCE(i.patient_age_group,''), COALESCE(i.patient_sex,''), i.incident_type_id, i.severity_level_id, i.priority_level_id, COALESCE(rpl.code,''), COALESCE(i.summary,''), COALESCE(i.description,''), i.district_id, i.facility_id, COALESCE(i.village,''), COALESCE(i.parish,''), COALESCE(i.subcounty,''), COALESCE(i.landmark,''), i.latitude, i.longitude, i.verification_status, i.status, i.reported_at, i.created_by_user_id, i.triaged_by_user_id, i.triaged_at, i.assigned_at, i.closed_at, i.created_at, i.updated_at FROM incidents i LEFT JOIN ref_priority_levels rpl ON rpl.id=i.priority_level_id %s %s LIMIT $%d OFFSET $%d`, whereSQL, platformdb.BuildOrderBy(p, map[string]string{"reported_at": "i.reported_at", "created_at": "i.created_at", "status": "i.status"}), pos, pos+1)
	rows, err := r.db.Query(ctx, q, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []incidentdomain.Incident{}
	for rows.Next() {
		var out incidentdomain.Incident
		if err := rows.Scan(&out.ID, &out.IncidentNumber, &out.SourceChannel, &out.CallerName, &out.CallerPhone, &out.PatientName, &out.PatientPhone, &out.PatientAgeGroup, &out.PatientSex, &out.IncidentTypeID, &out.SeverityLevelID, &out.PriorityLevelID, &out.PriorityCode, &out.Summary, &out.Description, &out.DistrictID, &out.FacilityID, &out.Village, &out.Parish, &out.Subcounty, &out.Landmark, &out.Latitude, &out.Longitude, &out.VerificationStatus, &out.Status, &out.ReportedAt, &out.CreatedByUserID, &out.TriagedByUserID, &out.TriagedAt, &out.AssignedAt, &out.ClosedAt, &out.CreatedAt, &out.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, out)
	}
	return items, total, rows.Err()
}

func (r *Repository) UpdateIncident(ctx context.Context, id string, req incidentapp.UpdateIncidentRequest) (incidentdomain.Incident, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	pos := 1

	var current incidentdomain.Incident
	var effectiveLat, effectiveLon *float64
	if req.Latitude != nil || req.Longitude != nil {
		var err error
		current, err = r.GetIncidentByID(ctx, id)
		if err != nil {
			return incidentdomain.Incident{}, err
		}
		effectiveLat = current.Latitude
		effectiveLon = current.Longitude
	}

	if req.SourceChannel != nil {
		sets = append(sets, fmt.Sprintf("source_channel = $%d", pos))
		args = append(args, strings.ToUpper(trimmedValue(req.SourceChannel)))
		pos++
	}
	if req.CallerName != nil {
		sets = append(sets, fmt.Sprintf("caller_name = $%d", pos))
		args = append(args, trimmedValue(req.CallerName))
		pos++
	}
	if req.CallerPhone != nil {
		sets = append(sets, fmt.Sprintf("caller_phone = $%d", pos))
		args = append(args, trimmedValue(req.CallerPhone))
		pos++
	}
	if req.PatientName != nil {
		sets = append(sets, fmt.Sprintf("patient_name = $%d", pos))
		args = append(args, trimmedValue(req.PatientName))
		pos++
	}
	if req.PatientPhone != nil {
		sets = append(sets, fmt.Sprintf("patient_phone = $%d", pos))
		args = append(args, trimmedValue(req.PatientPhone))
		pos++
	}
	if req.PatientAgeGroup != nil {
		sets = append(sets, fmt.Sprintf("patient_age_group = $%d", pos))
		args = append(args, trimmedValue(req.PatientAgeGroup))
		pos++
	}
	if req.PatientSex != nil {
		sets = append(sets, fmt.Sprintf("patient_sex = $%d", pos))
		args = append(args, strings.ToUpper(trimmedValue(req.PatientSex)))
		pos++
	}
	if req.IncidentTypeID != nil {
		sets = append(sets, fmt.Sprintf("incident_type_id = $%d", pos))
		args = append(args, trimmedValue(req.IncidentTypeID))
		pos++
	}
	if req.SeverityLevelID != nil {
		sets = append(sets, fmt.Sprintf("severity_level_id = $%d", pos))
		args = append(args, nilIfBlank(req.SeverityLevelID))
		pos++
	}
	if req.PriorityLevelID != nil {
		sets = append(sets, fmt.Sprintf("priority_level_id = $%d", pos))
		args = append(args, nilIfBlank(req.PriorityLevelID))
		pos++
	}
	if req.Summary != nil {
		sets = append(sets, fmt.Sprintf("summary = $%d", pos))
		args = append(args, trimmedValue(req.Summary))
		pos++
	}
	if req.Description != nil {
		sets = append(sets, fmt.Sprintf("description = $%d", pos))
		args = append(args, trimmedValue(req.Description))
		pos++
	}
	if req.DistrictID != nil {
		sets = append(sets, fmt.Sprintf("district_id = $%d", pos))
		args = append(args, nilIfBlank(req.DistrictID))
		pos++
	}
	if req.FacilityID != nil {
		sets = append(sets, fmt.Sprintf("facility_id = $%d", pos))
		args = append(args, nilIfBlank(req.FacilityID))
		pos++
	}
	if req.Village != nil {
		sets = append(sets, fmt.Sprintf("village = $%d", pos))
		args = append(args, trimmedValue(req.Village))
		pos++
	}
	if req.Parish != nil {
		sets = append(sets, fmt.Sprintf("parish = $%d", pos))
		args = append(args, trimmedValue(req.Parish))
		pos++
	}
	if req.Subcounty != nil {
		sets = append(sets, fmt.Sprintf("subcounty = $%d", pos))
		args = append(args, trimmedValue(req.Subcounty))
		pos++
	}
	if req.Landmark != nil {
		sets = append(sets, fmt.Sprintf("landmark = $%d", pos))
		args = append(args, trimmedValue(req.Landmark))
		pos++
	}
	if req.Latitude != nil {
		sets = append(sets, fmt.Sprintf("latitude = $%d", pos))
		args = append(args, *req.Latitude)
		effectiveLat = req.Latitude
		pos++
	}
	if req.Longitude != nil {
		sets = append(sets, fmt.Sprintf("longitude = $%d", pos))
		args = append(args, *req.Longitude)
		effectiveLon = req.Longitude
		pos++
	}
	if req.Latitude != nil || req.Longitude != nil {
		sets = append(sets, fmt.Sprintf("location = CASE WHEN $%d::float8 IS NULL OR $%d::float8 IS NULL THEN NULL ELSE ST_SetSRID(ST_MakePoint($%d, $%d), 4326)::geography END", pos, pos+1, pos+1, pos))
		var latArg any
		var lonArg any
		if effectiveLat != nil {
			latArg = *effectiveLat
		}
		if effectiveLon != nil {
			lonArg = *effectiveLon
		}
		args = append(args, latArg, lonArg)
		pos += 2
	}
	if req.VerificationStatus != nil {
		sets = append(sets, fmt.Sprintf("verification_status = $%d", pos))
		args = append(args, strings.ToUpper(trimmedValue(req.VerificationStatus)))
		pos++
	}
	if req.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", pos))
		args = append(args, strings.ToUpper(trimmedValue(req.Status)))
		sets = append(sets, fmt.Sprintf("closed_at = CASE WHEN $%d IN ('COMPLETED','CANCELLED','REJECTED') THEN now() ELSE closed_at END", pos))
		pos++
	}
	if req.ReportedAt != nil {
		sets = append(sets, fmt.Sprintf("reported_at = $%d", pos))
		args = append(args, req.ReportedAt.UTC())
		pos++
	}

	if len(sets) == 0 {
		return r.GetIncidentByID(ctx, id)
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf("UPDATE incidents SET %s WHERE id = $%d", strings.Join(sets, ", "), pos)
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return incidentdomain.Incident{}, err
	}
	return r.GetIncidentByID(ctx, id)
}

func (r *Repository) UpdateIncidentStatus(ctx context.Context, id, status string) (incidentdomain.Incident, error) {
	_, err := r.db.Exec(ctx, `UPDATE incidents SET status=$2, updated_at=now(), closed_at=CASE WHEN $2 IN ('COMPLETED','CANCELLED','REJECTED') THEN now() ELSE closed_at END WHERE id=$1`, id, status)
	if err != nil {
		return incidentdomain.Incident{}, err
	}
	return r.GetIncidentByID(ctx, id)
}

func (r *Repository) CreateIncidentUpdate(ctx context.Context, incidentID, updateType, oldValue, newValue, notes string, actorUserID *string) error {
	_, err := r.db.Exec(ctx, `INSERT INTO incident_updates (id, incident_id, update_type, old_value, new_value, notes, actor_user_id) VALUES (gen_random_uuid(),$1,$2,NULLIF($3,''),NULLIF($4,''),$5,$6)`, incidentID, updateType, oldValue, newValue, notes, actorUserID)
	return err
}

func (r *Repository) ResolvePriorityLevelIDByCode(ctx context.Context, code string) (*string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM ref_priority_levels WHERE code=$1`, strings.ToUpper(code)).Scan(&id)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

func (r *Repository) SetIncidentPriorityByCode(ctx context.Context, incidentID, code string) error {
	_, err := r.db.Exec(ctx, `UPDATE incidents i SET priority_level_id=rpl.id, updated_at=now() FROM ref_priority_levels rpl WHERE i.id=$1 AND rpl.code=$2`, incidentID, strings.ToUpper(code))
	return err
}
func (r *Repository) SetIncidentTriageSummary(ctx context.Context, incidentID string, triagedByUserID *string) error {
	_, err := r.db.Exec(ctx, `UPDATE incidents SET triaged_by_user_id=$2, triaged_at=now(), updated_at=now() WHERE id=$1`, incidentID, triagedByUserID)
	return err
}

func (r *Repository) ResolveQuestionnaireIDByCode(ctx context.Context, questionnaireCode string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT id FROM triage_questionnaires WHERE code=$1 AND is_active=TRUE`, strings.ToUpper(questionnaireCode)).Scan(&id)
	return id, err
}

func (r *Repository) GetQuestionDefinitions(ctx context.Context, questionnaireCode string) (map[string]incidentapp.QuestionDefinition, error) {
	rows, err := r.db.Query(ctx, `
		SELECT tq.id, tq.code, tq.response_type,
		       MAX(CASE WHEN tqo.option_value='true' THEN tqo.score END) AS true_score,
		       MAX(CASE WHEN tqo.option_value='false' THEN tqo.score END) AS false_score
		FROM triage_questions tq
		JOIN triage_questionnaires tqq ON tqq.id=tq.questionnaire_id
		LEFT JOIN triage_question_options tqo ON tqo.question_id=tq.id
		WHERE tqq.code=$1 AND tq.is_active=TRUE
		GROUP BY tq.id, tq.code, tq.response_type`, strings.ToUpper(questionnaireCode))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]incidentapp.QuestionDefinition{}
	for rows.Next() {
		var qid, code, respType string
		var trueScore, falseScore *int
		if err := rows.Scan(&qid, &code, &respType, &trueScore, &falseScore); err != nil {
			return nil, err
		}
		out[strings.ToUpper(code)] = incidentapp.QuestionDefinition{QuestionID: qid, ResponseType: respType, TrueScore: trueScore, FalseScore: falseScore}
	}
	return out, rows.Err()
}

func (r *Repository) CreatePersistedTriageSession(ctx context.Context, session incidentdomain.PersistedTriageSession) (incidentdomain.PersistedTriageSession, error) {
	err := platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `INSERT INTO incident_triage_sessions (id, incident_id, questionnaire_id, triage_mode, total_score, boolean_true_count, auto_dispatch_eligible, derived_priority_level_id, notes, triaged_by_user_id, triaged_at, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now())`, session.ID, session.IncidentID, session.QuestionnaireID, session.TriageMode, session.TotalScore, session.BooleanTrueCount, session.AutoDispatchEligible, session.DerivedPriorityLevelID, session.Notes, session.TriagedByUserID, session.TriagedAt)
		if err != nil {
			return err
		}
		for _, resp := range session.Responses {
			_, err := tx.Exec(ctx, `INSERT INTO incident_triage_responses (id, triage_session_id, incident_id, question_id, question_code, response_type, response_value_text, response_value_bool, response_value_int, selected_option_id, selected_option_code, score_awarded, created_at, updated_at) VALUES (gen_random_uuid(),$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,now(),now())`, session.ID, session.IncidentID, resp.QuestionID, resp.QuestionCode, resp.ResponseType, resp.ResponseValueText, resp.ResponseValueBool, resp.ResponseValueInt, resp.SelectedOptionID, resp.SelectedOptionCode, resp.ScoreAwarded)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return incidentdomain.PersistedTriageSession{}, err
	}
	session.CreatedAt = time.Now().UTC()
	session.UpdatedAt = session.CreatedAt
	return session, nil
}
