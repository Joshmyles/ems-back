package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dispatch/internal/modules/users/application/dto"
	"dispatch/internal/modules/users/domain"
	"dispatch/internal/platform/db"

	userapp "dispatch/internal/modules/users/application"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, u domain.User, passwordHash string, profile dto.CreateUserRequest) error {
	return db.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			INSERT INTO users (
				id, staff_no, username, first_name, last_name, other_name, gender,
				phone, email, password_hash, status, is_active, preferred_language,
				timezone, created_at, updated_at
			)
			VALUES (
				$1,
				COALESCE(NULLIF($2, ''), 'EMS-' || lpad(nextval('users_staff_no_seq')::text, 6, '0')),
				$3,$4,$5,$6,$7,
				$8,$9,$10,'ACTIVE',true,$11,
				$12,$13,$13
			)
		`,
			u.ID, profile.StaffNo, u.Username, u.FirstName, u.LastName, profile.OtherName, profile.Gender,
			u.Phone, u.Email, passwordHash,
			coalesce(profile.PreferredLanguage, "en"),
			coalesce(profile.Timezone, "Africa/Kampala"),
			u.CreatedAt,
		)
		if err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO user_profiles (
				user_id, cadre, license_number, specialization, date_of_birth,
				national_id, avatar_url, emergency_contact_name, emergency_contact_phone,
				address, created_at, updated_at
			)
			VALUES (
				$1,$2,$3,$4,$5,
				$6,$7,$8,$9,
				$10,now(),now()
			)
		`,
			u.ID, profile.Cadre, profile.LicenseNumber, profile.Specialization, profile.DateOfBirth,
			profile.NationalID, profile.AvatarURL, profile.EmergencyContactName, profile.EmergencyContactPhone,
			profile.Address,
		)
		return err
	})
}

func coalesce(v string, fallback string) string {
	if strings.TrimSpace(v) == "" {
		return fallback
	}
	return v
}

func (r *Repository) List(ctx context.Context, params dto.ListUsersParams) ([]domain.User, int64, error) {
	p := params.Pagination
	allowedSorts := map[string]string{
		"created_at": "u.created_at",
		"username":   "u.username",
		"first_name": "u.first_name",
		"last_name":  "u.last_name",
		"status":     "u.status",
	}

	baseWhere := []string{"u.deleted_at IS NULL"}
	args := make([]any, 0)
	argPos := 1

	if p.Search != "" {
		baseWhere = append(baseWhere, fmt.Sprintf("(u.username ILIKE $%d OR u.first_name ILIKE $%d OR u.last_name ILIKE $%d OR COALESCE(u.email, '') ILIKE $%d OR COALESCE(u.phone, '') ILIKE $%d)", argPos, argPos, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	for key, value := range p.Filters {
		switch key {
		case "status":
			baseWhere = append(baseWhere, fmt.Sprintf("u.status = $%d", argPos))
			args = append(args, strings.ToUpper(value))
			argPos++
		case "is_active":
			baseWhere = append(baseWhere, fmt.Sprintf("u.is_active::text = $%d", argPos))
			args = append(args, strings.ToLower(value))
			argPos++
		}
	}

	whereSQL := "WHERE " + strings.Join(baseWhere, " AND ")
	countSQL := fmt.Sprintf(`SELECT COUNT(1) FROM users u %s`, whereSQL)

	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := db.BuildOrderBy(p, allowedSorts)
	listSQL := fmt.Sprintf(`
		SELECT u.id, u.username, u.first_name, u.last_name, COALESCE(u.phone,''), COALESCE(u.email,''), u.status, u.created_at
		FROM users u
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, argPos, argPos+1)

	listArgs := append(args, p.PageSize, p.Offset)
	rows, err := r.db.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []domain.User
	for rows.Next() {
		var u domain.User
		if err := rows.Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, &u.Phone, &u.Email, &u.Status, &u.CreatedAt); err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string) (domain.User, error) {
	var u domain.User
	err := r.db.QueryRow(ctx, `
		SELECT id, username, first_name, last_name, COALESCE(phone,''), COALESCE(email,''), status, created_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, id).Scan(&u.ID, &u.Username, &u.FirstName, &u.LastName, &u.Phone, &u.Email, &u.Status, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, userapp.ErrUserNotFound
	}
	return u, err
}

func (r *Repository) Update(ctx context.Context, id string, req dto.UpdateUserRequest) (domain.User, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	pos := 1

	if req.FirstName != nil {
		sets = append(sets, fmt.Sprintf("first_name = $%d", pos))
		args = append(args, *req.FirstName)
		pos++
	}
	if req.LastName != nil {
		sets = append(sets, fmt.Sprintf("last_name = $%d", pos))
		args = append(args, *req.LastName)
		pos++
	}
	if req.Phone != nil {
		sets = append(sets, fmt.Sprintf("phone = $%d", pos))
		args = append(args, *req.Phone)
		pos++
	}
	if req.Email != nil {
		sets = append(sets, fmt.Sprintf("email = $%d", pos))
		args = append(args, *req.Email)
		pos++
	}
	if req.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", pos))
		args = append(args, strings.ToUpper(*req.Status))
		pos++
	}
	if req.IsActive != nil {
		sets = append(sets, fmt.Sprintf("is_active = $%d", pos))
		args = append(args, *req.IsActive)
		pos++
	}

	if len(sets) == 0 {
		return r.GetByID(ctx, id)
	}

	sets = append(sets, "updated_at = now()")
	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE users
		SET %s
		WHERE id = $%d AND deleted_at IS NULL
	`, strings.Join(sets, ", "), pos)

	ct, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			return domain.User{}, err
		}
		return domain.User{}, err
	}
	if ct.RowsAffected() == 0 {
		return domain.User{}, userapp.ErrUserNotFound
	}

	return r.GetByID(ctx, id)
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE users
		SET deleted_at = now(),
		    is_active = FALSE,
		    updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, id)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return userapp.ErrUserNotFound
	}
	return nil
}

func (r *Repository) GetPasswordHash(ctx context.Context, userID string) (string, error) {
	var hash string
	err := r.db.QueryRow(ctx, `
		SELECT password_hash
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
	`, userID).Scan(&hash)
	return hash, err
}

func (r *Repository) ChangePassword(ctx context.Context, userID, newHash string) error {
	ct, err := r.db.Exec(ctx, `
		UPDATE users
		SET password_hash = $2,
		    password_changed_at = now(),
		    failed_login_attempts = 0,
		    is_locked = FALSE,
		    updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
	`, userID, newHash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return userapp.ErrUserNotFound
	}
	return nil
}

func (r *Repository) AssignRole(ctx context.Context, userID string, req dto.AssignRoleRequest) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO user_roles (
			id, user_id, role_id, scope_type, scope_id, active, assigned_at, assigned_by
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, TRUE, now(), $5)
		ON CONFLICT (user_id, role_id, scope_type, scope_id)
		DO UPDATE SET active = TRUE, assigned_at = now(), assigned_by = EXCLUDED.assigned_by
	`, userID, req.RoleID, req.ScopeType, req.ScopeID, req.AssignedBy)
	return err
}

func (r *Repository) RemoveRole(ctx context.Context, userID string, roleID string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_roles
		SET active = FALSE
		WHERE user_id = $1 AND role_id = $2
	`, userID, roleID)
	return err
}

func (r *Repository) ListRoles(ctx context.Context, userID string) ([]dto.UserRoleResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT ur.id, r.id, r.code, r.name, ur.scope_type, ur.scope_id::text, ur.active
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []dto.UserRoleResponse
	for rows.Next() {
		var x dto.UserRoleResponse
		if err := rows.Scan(&x.ID, &x.RoleID, &x.RoleCode, &x.RoleName, &x.ScopeType, &x.ScopeID, &x.Active); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (r *Repository) UpdateAssignment(ctx context.Context, assignmentID string, req dto.AssignUserRequest) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_assignments
		SET district_id = $2,
		    subcounty_id = $3,
		    facility_id = $4,
		    assignment_level = $5,
		    team_name = $6,
		    is_primary = $7,
		    active = $8,
		    start_date = COALESCE($9::date, start_date),
		    end_date = $10,
		    updated_at = now()
		WHERE id = $1
	`, assignmentID, req.DistrictID, req.SubcountyID, req.FacilityID,
		req.AssignmentLevel, req.TeamName, req.IsPrimary, req.Active, req.StartDate, req.EndDate)
	return err
}

func (r *Repository) ListAssignments(ctx context.Context, userID string) ([]dto.UserAssignmentResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, district_id::text, subcounty_id::text, facility_id::text,
		       assignment_level, COALESCE(team_name,''), is_primary, active, start_date, end_date
		FROM user_assignments
		WHERE user_id = $1
		ORDER BY is_primary DESC, created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []dto.UserAssignmentResponse
	for rows.Next() {
		var x dto.UserAssignmentResponse
		if err := rows.Scan(
			&x.ID, &x.DistrictID, &x.SubcountyID, &x.FacilityID,
			&x.AssignmentLevel, &x.TeamName, &x.IsPrimary, &x.Active, &x.StartDate, &x.EndDate,
		); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (r *Repository) AssignCapability(ctx context.Context, userID string, req dto.AssignCapabilityRequest) error {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO user_capabilities (
			id, user_id, capability_id, level_no, is_active, expires_at, created_at
		)
		VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, now())
		ON CONFLICT (user_id, capability_id)
		DO UPDATE SET
			level_no = EXCLUDED.level_no,
			is_active = EXCLUDED.is_active,
			expires_at = EXCLUDED.expires_at
	`, userID, req.CapabilityID, req.LevelNo, isActive, req.ExpiresAt)
	return err
}

func (r *Repository) UpdateCapability(ctx context.Context, capabilityRecordID string, req dto.AssignCapabilityRequest) error {
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	_, err := r.db.Exec(ctx, `
		UPDATE user_capabilities
		SET capability_id = $2,
		    level_no = $3,
		    is_active = $4,
		    expires_at = $5
		WHERE id = $1
	`, capabilityRecordID, req.CapabilityID, req.LevelNo, isActive, req.ExpiresAt)
	return err
}

func (r *Repository) ListCapabilities(ctx context.Context, userID string) ([]dto.UserCapabilityResponse, error) {
	rows, err := r.db.Query(ctx, `
		SELECT uc.id, rc.id, rc.code, rc.name, uc.level_no, uc.is_active, uc.expires_at
		FROM user_capabilities uc
		JOIN ref_capabilities rc ON rc.id = uc.capability_id
		WHERE uc.user_id = $1
		ORDER BY rc.name
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []dto.UserCapabilityResponse
	for rows.Next() {
		var x dto.UserCapabilityResponse
		if err := rows.Scan(
			&x.ID, &x.CapabilityID, &x.CapabilityCode, &x.CapabilityName,
			&x.LevelNo, &x.IsActive, &x.ExpiresAt,
		); err != nil {
			return nil, err
		}
		items = append(items, x)
	}
	return items, rows.Err()
}

func (r *Repository) GetProfile(ctx context.Context, userID string) (map[string]any, error) {
	row := r.db.QueryRow(ctx, `
		SELECT cadre, license_number, specialization, date_of_birth, national_id,
		       avatar_url, emergency_contact_name, emergency_contact_phone, address
		FROM user_profiles
		WHERE user_id = $1
	`, userID)

	var cadre, licenseNumber, specialization, nationalID, avatarURL, emergencyName, emergencyPhone, address string
	var dob *time.Time

	err := row.Scan(
		&cadre, &licenseNumber, &specialization, &dob, &nationalID,
		&avatarURL, &emergencyName, &emergencyPhone, &address,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return map[string]any{
				"cadre":                   "",
				"license_number":          "",
				"specialization":          "",
				"date_of_birth":           nil,
				"national_id":             "",
				"avatar_url":              "",
				"emergency_contact_name":  "",
				"emergency_contact_phone": "",
				"address":                 "",
			}, nil
		}
		return nil, err
	}

	return map[string]any{
		"cadre":                   cadre,
		"license_number":          licenseNumber,
		"specialization":          specialization,
		"date_of_birth":           dob,
		"national_id":             nationalID,
		"avatar_url":              avatarURL,
		"emergency_contact_name":  emergencyName,
		"emergency_contact_phone": emergencyPhone,
		"address":                 address,
	}, nil
}

func (r *Repository) UpdateProfile(ctx context.Context, userID string, req dto.UpdateUserProfileRequest) error {
	_, err := r.db.Exec(ctx, `
		UPDATE user_profiles
		SET cadre = COALESCE($2, cadre),
		    license_number = COALESCE($3, license_number),
		    specialization = COALESCE($4, specialization),
		    date_of_birth = COALESCE($5::date, date_of_birth),
		    national_id = COALESCE($6, national_id),
		    avatar_url = COALESCE($7, avatar_url),
		    emergency_contact_name = COALESCE($8, emergency_contact_name),
		    emergency_contact_phone = COALESCE($9, emergency_contact_phone),
		    address = COALESCE($10, address),
		    updated_at = now()
		WHERE user_id = $1
	`, userID, req.Cadre, req.LicenseNumber, req.Specialization, req.DateOfBirth,
		req.NationalID, req.AvatarURL, req.EmergencyContactName, req.EmergencyContactPhone, req.Address)
	return err
}

func (r *Repository) AssignUser(ctx context.Context, userID string, req dto.AssignUserRequest) error {
	return db.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		if req.IsPrimary {
			_, err := tx.Exec(ctx, `
				UPDATE user_assignments
				SET is_primary = FALSE, updated_at = now()
				WHERE user_id = $1 AND active = TRUE
			`, userID)
			if err != nil {
				return err
			}
		}

		active := req.Active
		if !req.Active {
			active = false
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO user_assignments (
				id, user_id, district_id, subcounty_id, facility_id,
				assignment_level, team_name, is_primary, active,
				start_date, end_date, created_at, updated_at
			)
			VALUES (
				gen_random_uuid(), $1, $2, $3, $4,
				$5, $6, $7, $8,
				COALESCE($9::date, CURRENT_DATE), $10, now(), now()
			)
		`,
			userID,
			req.DistrictID,
			req.SubcountyID,
			req.FacilityID,
			req.AssignmentLevel,
			req.TeamName,
			req.IsPrimary,
			active,
			req.StartDate,
			req.EndDate,
		)
		return err
	})
}
