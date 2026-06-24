package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"dispatch/internal/modules/reference/application/dto"
	refdomain "dispatch/internal/modules/reference/domain"
	platformdb "dispatch/internal/platform/db"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListDistricts(ctx context.Context, params dto.ListDistrictsParams) ([]refdomain.District, int64, error) {
	p := params.Pagination
	allowedSorts := map[string]string{
		"name":       "d.name",
		"region":     "d.region",
		"created_at": "d.created_at",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(d.name ILIKE $%d OR COALESCE(d.region,'') ILIKE $%d OR COALESCE(d.code,'') ILIKE $%d)`, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	if v, ok := p.Filters["is_active"]; ok {
		where = append(where, fmt.Sprintf(`d.is_active::text = $%d`, argPos))
		args = append(args, strings.ToLower(v))
		argPos++
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM ref_districts d `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)
	query := fmt.Sprintf(`
		SELECT d.id, COALESCE(d.code,''), d.name, COALESCE(d.region,''), d.is_active, d.created_at, d.updated_at
		FROM ref_districts d
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]refdomain.District, 0)
	for rows.Next() {
		var item refdomain.District
		if err := rows.Scan(
			&item.ID, &item.Code, &item.Name, &item.Region,
			&item.IsActive, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

func (r *Repository) ListSubcounties(ctx context.Context, params dto.ListSubcountiesParams) ([]refdomain.Subcounty, int64, error) {
	p := params.Pagination
	allowedSorts := map[string]string{
		"name":       "s.name",
		"created_at": "s.created_at",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if params.DistrictID != nil && *params.DistrictID != "" {
		where = append(where, fmt.Sprintf(`s.district_id = $%d`, argPos))
		args = append(args, *params.DistrictID)
		argPos++
	}

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(s.name ILIKE $%d OR COALESCE(s.code,'') ILIKE $%d)`, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	if v, ok := p.Filters["is_active"]; ok {
		where = append(where, fmt.Sprintf(`s.is_active::text = $%d`, argPos))
		args = append(args, strings.ToLower(v))
		argPos++
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM ref_subcounties s `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)
	query := fmt.Sprintf(`
		SELECT s.id, s.district_id, COALESCE(s.code,''), s.name, s.is_active, s.created_at, s.updated_at
		FROM ref_subcounties s
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]refdomain.Subcounty, 0)
	for rows.Next() {
		var item refdomain.Subcounty
		if err := rows.Scan(
			&item.ID, &item.DistrictID, &item.Code, &item.Name,
			&item.IsActive, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

func (r *Repository) ListFacilities(ctx context.Context, params dto.ListFacilitiesParams) ([]refdomain.Facility, int64, error) {
	p := params.Pagination
	allowedSorts := map[string]string{
		"name":       "f.name",
		"created_at": "f.created_at",
		"ownership":  "f.ownership",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if params.DistrictID != nil && *params.DistrictID != "" {
		where = append(where, fmt.Sprintf(`f.district_id = $%d`, argPos))
		args = append(args, *params.DistrictID)
		argPos++
	}

	if params.SubcountyID != nil && *params.SubcountyID != "" {
		where = append(where, fmt.Sprintf(`f.subcounty_id = $%d`, argPos))
		args = append(args, *params.SubcountyID)
		argPos++
	}

	if params.LevelID != nil && *params.LevelID != "" {
		where = append(where, fmt.Sprintf(`f.level_id = $%d`, argPos))
		args = append(args, *params.LevelID)
		argPos++
	}

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			f.name ILIKE $%d OR
			COALESCE(f.short_name,'') ILIKE $%d OR
			COALESCE(f.nhfr_id,'') ILIKE $%d OR
			COALESCE(f.code,'') ILIKE $%d
		)`, argPos, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	if v, ok := p.Filters["is_active"]; ok {
		where = append(where, fmt.Sprintf(`f.is_active::text = $%d`, argPos))
		args = append(args, strings.ToLower(v))
		argPos++
	}

	if v, ok := p.Filters["is_dispatch_station"]; ok {
		where = append(where, fmt.Sprintf(`f.is_dispatch_station::text = $%d`, argPos))
		args = append(args, strings.ToLower(v))
		argPos++
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM ref_facilities f `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)
	query := fmt.Sprintf(`
		SELECT
			f.id, f.code, f.name, COALESCE(f.short_name,''), COALESCE(f.nhfr_id,''),
			f.district_id, COALESCE(d.name,''), f.subcounty_id, COALESCE(s.name,''),
			f.level_id, COALESCE(fl.name,''), COALESCE(f.ownership,''), COALESCE(f.phone,''),
			COALESCE(f.email,''), COALESCE(f.address,''), f.latitude, f.longitude,
			f.is_dispatch_station, f.is_active, f.created_at, f.updated_at
		FROM ref_facilities f
		LEFT JOIN ref_districts d ON d.id = f.district_id
		LEFT JOIN ref_subcounties s ON s.id = f.subcounty_id
		LEFT JOIN ref_facility_levels fl ON fl.id = f.level_id
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]refdomain.Facility, 0)
	for rows.Next() {
		var item refdomain.Facility
		if err := rows.Scan(
			&item.ID, &item.Code, &item.Name, &item.ShortName, &item.NHFRID,
			&item.DistrictID, &item.DistrictName, &item.SubcountyID, &item.SubcountyName,
			&item.LevelID, &item.LevelName, &item.Ownership, &item.Phone,
			&item.Email, &item.Address, &item.Latitude, &item.Longitude,
			&item.IsDispatchStation, &item.IsActive, &item.CreatedAt, &item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

func (r *Repository) ListFacilityLevels(ctx context.Context) ([]refdomain.FacilityLevel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, rank_no, is_active, created_at
		FROM ref_facility_levels
		WHERE is_active = TRUE
		ORDER BY rank_no ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]refdomain.FacilityLevel, 0)
	for rows.Next() {
		var item refdomain.FacilityLevel
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.RankNo, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListIncidentTypes(ctx context.Context) ([]refdomain.IncidentType, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, COALESCE(description,''), requires_transport, is_active, created_at
		FROM ref_incident_types
		WHERE is_active = TRUE
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]refdomain.IncidentType, 0)
	for rows.Next() {
		var item refdomain.IncidentType
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.Description, &item.RequiresTransport, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListPriorityLevels(ctx context.Context) ([]refdomain.PriorityLevel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, color_code, sort_order, target_response_minutes, severity_weight,
		       COALESCE(escalation_note,''), is_active, created_at
		FROM ref_priority_levels
		WHERE is_active = TRUE
		ORDER BY sort_order ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]refdomain.PriorityLevel, 0)
	for rows.Next() {
		var item refdomain.PriorityLevel
		if err := rows.Scan(
			&item.ID, &item.Code, &item.Name, &item.ColorCode, &item.SortOrder,
			&item.TargetResponseMinutes, &item.SeverityWeight, &item.EscalationNote,
			&item.IsActive, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListSeverityLevels(ctx context.Context) ([]refdomain.SeverityLevel, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, sort_order, is_active, created_at
		FROM ref_severity_levels
		WHERE is_active = TRUE
		ORDER BY sort_order ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]refdomain.SeverityLevel, 0)
	for rows.Next() {
		var item refdomain.SeverityLevel
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.SortOrder, &item.IsActive, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListAmbulanceCategories(ctx context.Context) ([]refdomain.AmbulanceCategory, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, COALESCE(description,''), supports_maternal, supports_neonatal,
		       supports_trauma, supports_critical_care, supports_referral, min_crew_count,
		       is_active, created_at
		FROM ref_ambulance_categories
		WHERE is_active = TRUE
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]refdomain.AmbulanceCategory, 0)
	for rows.Next() {
		var item refdomain.AmbulanceCategory
		if err := rows.Scan(
			&item.ID, &item.Code, &item.Name, &item.Description, &item.SupportsMaternal,
			&item.SupportsNeonatal, &item.SupportsTrauma, &item.SupportsCriticalCare,
			&item.SupportsReferral, &item.MinCrewCount, &item.IsActive, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListCapabilities(ctx context.Context) ([]refdomain.Capability, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, code, name, COALESCE(description,''), capability_type, is_active, created_at
		FROM ref_capabilities
		WHERE is_active = TRUE
		ORDER BY name ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]refdomain.Capability, 0)
	for rows.Next() {
		var item refdomain.Capability
		if err := rows.Scan(
			&item.ID, &item.Code, &item.Name, &item.Description,
			&item.CapabilityType, &item.IsActive, &item.CreatedAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) ListTriageQuestions(ctx context.Context, params dto.ListTriageQuestionsParams) ([]refdomain.TriageQuestion, int64, error) {
	p := params.Pagination
	allowedSorts := map[string]string{
		"display_order": "tq.display_order",
		"created_at":    "tq.created_at",
		"code":          "tq.code",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if params.QuestionnaireCode != nil && *params.QuestionnaireCode != "" {
		where = append(where, fmt.Sprintf(`tqq.code = $%d`, argPos))
		args = append(args, strings.ToUpper(*params.QuestionnaireCode))
		argPos++
	}

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			tq.code ILIKE $%d OR
			tq.question_text ILIKE $%d OR
			tq.response_type ILIKE $%d
		)`, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	if v, ok := p.Filters["is_active"]; ok {
		where = append(where, fmt.Sprintf(`tq.is_active::text = $%d`, argPos))
		args = append(args, strings.ToLower(v))
		argPos++
	}

	if v, ok := p.Filters["is_required"]; ok {
		where = append(where, fmt.Sprintf(`tq.is_required::text = $%d`, argPos))
		args = append(args, strings.ToLower(v))
		argPos++
	}

	if v, ok := p.Filters["response_type"]; ok {
		where = append(where, fmt.Sprintf(`tq.response_type = $%d`, argPos))
		args = append(args, strings.ToUpper(v))
		argPos++
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM triage_questions tq
		JOIN triage_questionnaires tqq ON tqq.id = tq.questionnaire_id
		`+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Append a unique tiebreaker (display_order, then the UNIQUE code) so OFFSET
	// pagination is deterministic. The default sort column (created_at) ties
	// across rows inserted in the same statement, which made pages overlap and
	// silently drop questions.
	orderBy := platformdb.BuildOrderBy(p, allowedSorts) + ", tq.display_order ASC, tq.code ASC"
	query := fmt.Sprintf(`
		SELECT
			tq.id,
			tq.questionnaire_id,
			tqq.code,
			tq.code,
			tq.question_text,
			tq.response_type,
			tq.display_order,
			tq.is_required,
			tq.is_active,
			tq.created_at
		FROM triage_questions tq
		JOIN triage_questionnaires tqq ON tqq.id = tq.questionnaire_id
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]refdomain.TriageQuestion, 0)
	for rows.Next() {
		var item refdomain.TriageQuestion
		if err := rows.Scan(
			&item.ID,
			&item.QuestionnaireID,
			&item.QuestionnaireCode,
			&item.Code,
			&item.QuestionText,
			&item.ResponseType,
			&item.DisplayOrder,
			&item.IsRequired,
			&item.IsActive,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}

func (r *Repository) ListRoles(ctx context.Context, params dto.ListRolesParams) ([]refdomain.Role, int64, error) {
	p := params.Pagination

	allowedSorts := map[string]string{
		"name":       "r.name",
		"code":       "r.code",
		"created_at": "r.created_at",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			r.name ILIKE $%d OR
			r.code ILIKE $%d OR
			COALESCE(r.description,'') ILIKE $%d
		)`, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	if v, ok := p.Filters["is_system"]; ok {
		where = append(where, fmt.Sprintf(`r.is_system::text = $%d`, argPos))
		args = append(args, strings.ToLower(v))
		argPos++
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	// total
	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(1) FROM roles r `+whereSQL,
		args...,
	).Scan(&total); err != nil {
		return nil, 0, err
	}

	// query
	query := fmt.Sprintf(`
		SELECT id, code, name, description, is_system, created_at, updated_at
		FROM roles r
		%s
		%s
		LIMIT $%d OFFSET $%d
	`,
		whereSQL,
		platformdb.BuildOrderBy(p, allowedSorts),
		argPos,
		argPos+1,
	)

	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := []refdomain.Role{}
	for rows.Next() {
		var item refdomain.Role
		if err := rows.Scan(
			&item.ID,
			&item.Code,
			&item.Name,
			&item.Description,
			&item.IsSystem,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}

	return items, total, rows.Err()
}
