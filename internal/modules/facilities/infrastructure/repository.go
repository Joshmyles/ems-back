package infrastructure

import (
	"context"
	"fmt"
	"strings"

	"dispatch/internal/modules/facilities/application"
	"dispatch/internal/modules/facilities/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const focalPersonRoleCode = "FACILITY_FOCAL_PERSON"

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ application.Repository = (*Repository)(nil)

func (r *Repository) ListFacilities(ctx context.Context, p platformdb.Pagination) ([]domain.Facility, int64, error) {
	allowedSorts := map[string]string{
		"created_at":    "f.facility", // default sort by facility name
		"facility":      "f.facility",
		"level":         "f.level",
		"ownership":     "f.ownership",
		"region":        "r.region",
		"district":      "d.district",
		"subcounty":     "s.subcounty",
		"facility_uid":  "f.facility_uid",
		"subcounty_uid": "f.subcounty_uid",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if p.Search != "" {
		where = append(where, fmt.Sprintf(`(
			LOWER(f.facility) LIKE LOWER($%d) OR
			LOWER(d.district) LIKE LOWER($%d) OR
			LOWER(s.subcounty) LIKE LOWER($%d) OR
			LOWER(r.region) LIKE LOWER($%d)
		)`, argPos, argPos, argPos, argPos))
		args = append(args, "%"+p.Search+"%")
		argPos++
	}

	for key, value := range p.Filters {
		switch key {
		case "region_uid":
			where = append(where, fmt.Sprintf("r.region_uid = $%d", argPos))
			args = append(args, value)
			argPos++
		case "district_uid":
			where = append(where, fmt.Sprintf("d.district_uid = $%d", argPos))
			args = append(args, value)
			argPos++
		case "subcounty_uid":
			where = append(where, fmt.Sprintf("s.subcounty_uid = $%d", argPos))
			args = append(args, value)
			argPos++
		case "level":
			where = append(where, fmt.Sprintf("LOWER(f.level) = LOWER($%d)", argPos))
			args = append(args, value)
			argPos++
		case "ownership":
			where = append(where, fmt.Sprintf("LOWER(f.ownership) = LOWER($%d)", argPos))
			args = append(args, value)
			argPos++
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	countSQL := fmt.Sprintf(`
		SELECT COUNT(1)
		FROM facilities f
		JOIN subcounties s ON s.subcounty_uid = f.subcounty_uid
		JOIN districts d ON d.district_uid = s.district_uid
		JOIN regions r ON r.region_uid = d.region_uid
		%s
	`, whereSQL)

	var total int64
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)

	listSQL := fmt.Sprintf(`
		SELECT
			f.facility_uid,
			f.subcounty_uid,
			f.facility,
			COALESCE(f.level, ''),
			COALESCE(f.ownership, ''),
			r.region_uid,
			d.district_uid,
			r.region,
			d.district,
			s.subcounty,
			f.focal_person_id::text,
			u.username,
			u.first_name,
			u.last_name,
			u.phone,
			u.email
		FROM facilities f
		JOIN subcounties s ON s.subcounty_uid = f.subcounty_uid
		JOIN districts d ON d.district_uid = s.district_uid
		JOIN regions r ON r.region_uid = d.region_uid
		LEFT JOIN users u ON u.id = f.focal_person_id
		%s
		%s
		LIMIT $%d OFFSET $%d
	`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, listSQL, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.Facility, 0)
	for rows.Next() {
		var f domain.Facility
		var focalID, focalUsername, focalFirst, focalLast, focalPhone, focalEmail *string
		if err := rows.Scan(
			&f.FacilityUID,
			&f.SubcountyUID,
			&f.Facility,
			&f.Level,
			&f.Ownership,
			&f.RegionUID,
			&f.DistrictUID,
			&f.Region,
			&f.District,
			&f.Subcounty,
			&focalID,
			&focalUsername,
			&focalFirst,
			&focalLast,
			&focalPhone,
			&focalEmail,
		); err != nil {
			return nil, 0, err
		}
		if focalID != nil {
			f.FocalPerson = &domain.FocalPerson{
				UserID:    *focalID,
				Username:  derefString(focalUsername),
				FirstName: derefString(focalFirst),
				LastName:  derefString(focalLast),
				Phone:     derefString(focalPhone),
				Email:     derefString(focalEmail),
			}
		}
		items = append(items, f)
	}

	return items, total, rows.Err()
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (r *Repository) GetByUID(ctx context.Context, uid string) (domain.Facility, error) {
	const q = `
SELECT f.facility_uid, f.subcounty_uid, f.facility, COALESCE(f.level,''), COALESCE(f.ownership,''),
       r.region_uid, d.district_uid, r.region, d.district, s.subcounty,
       f.focal_person_id::text, u.username, u.first_name, u.last_name, u.phone, u.email
FROM facilities f
JOIN subcounties s ON s.subcounty_uid = f.subcounty_uid
JOIN districts d ON d.district_uid = s.district_uid
JOIN regions r ON r.region_uid = d.region_uid
LEFT JOIN users u ON u.id = f.focal_person_id
WHERE f.facility_uid = $1`
	var f domain.Facility
	var focalID, focalUsername, focalFirst, focalLast, focalPhone, focalEmail *string
	err := r.db.QueryRow(ctx, q, uid).Scan(
		&f.FacilityUID,
		&f.SubcountyUID,
		&f.Facility,
		&f.Level,
		&f.Ownership,
		&f.RegionUID,
		&f.DistrictUID,
		&f.Region,
		&f.District,
		&f.Subcounty,
		&focalID,
		&focalUsername,
		&focalFirst,
		&focalLast,
		&focalPhone,
		&focalEmail,
	)
	if err != nil {
		return f, err
	}
	if focalID != nil {
		f.FocalPerson = &domain.FocalPerson{
			UserID:    *focalID,
			Username:  derefString(focalUsername),
			FirstName: derefString(focalFirst),
			LastName:  derefString(focalLast),
			Phone:     derefString(focalPhone),
			Email:     derefString(focalEmail),
		}
	}
	return f, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Facility) (domain.Facility, error) {
	const q = `
INSERT INTO facilities (facility_uid, subcounty_uid, facility, level, ownership)
VALUES ($1,$2,$3,$4,$5)
ON CONFLICT (facility_uid) DO NOTHING`
	if _, err := r.db.Exec(ctx, q,
		in.FacilityUID, in.SubcountyUID, in.Facility, in.Level, in.Ownership,
	); err != nil {
		return domain.Facility{}, err
	}
	return r.GetByUID(ctx, in.FacilityUID)
}

func (r *Repository) Update(ctx context.Context, uid string, req application.UpdateFacilityRequest) (domain.Facility, error) {
	sets := make([]string, 0)
	args := make([]any, 0)
	pos := 1

	if req.SubcountyUID != nil {
		sets = append(sets, fmt.Sprintf("subcounty_uid = $%d", pos))
		args = append(args, *req.SubcountyUID)
		pos++
	}
	if req.Facility != nil {
		sets = append(sets, fmt.Sprintf("facility = $%d", pos))
		args = append(args, *req.Facility)
		pos++
	}
	if req.Level != nil {
		sets = append(sets, fmt.Sprintf("level = $%d", pos))
		args = append(args, *req.Level)
		pos++
	}
	if req.Ownership != nil {
		sets = append(sets, fmt.Sprintf("ownership = $%d", pos))
		args = append(args, *req.Ownership)
		pos++
	}
	if len(sets) == 0 {
		return r.GetByUID(ctx, uid)
	}
	args = append(args, uid)
	query := fmt.Sprintf("UPDATE facilities SET %s WHERE facility_uid = $%d", strings.Join(sets, ", "), pos)
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return domain.Facility{}, err
	}
	return r.GetByUID(ctx, uid)
}

func (r *Repository) Delete(ctx context.Context, uid string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM facilities WHERE facility_uid = $1`, uid)
	return err
}

func (r *Repository) SetFocalPerson(ctx context.Context, facilityUID string, userID string) (domain.Facility, error) {
	err := platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		var previousUserID *string
		if err := tx.QueryRow(ctx,
			`SELECT focal_person_id::text FROM facilities WHERE facility_uid = $1 FOR UPDATE`,
			facilityUID,
		).Scan(&previousUserID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`UPDATE facilities SET focal_person_id = $1 WHERE facility_uid = $2`,
			userID, facilityUID,
		); err != nil {
			return err
		}

		if previousUserID != nil && *previousUserID != userID {
			if err := deactivateFacilityFocalRole(ctx, tx, *previousUserID, facilityUID); err != nil {
				return err
			}
		}

		return upsertFacilityFocalRole(ctx, tx, userID, facilityUID)
	})
	if err != nil {
		return domain.Facility{}, err
	}
	return r.GetByUID(ctx, facilityUID)
}

func (r *Repository) ClearFocalPerson(ctx context.Context, facilityUID string) (domain.Facility, error) {
	err := platformdb.WithTx(ctx, r.db, func(tx pgx.Tx) error {
		var previousUserID *string
		if err := tx.QueryRow(ctx,
			`SELECT focal_person_id::text FROM facilities WHERE facility_uid = $1 FOR UPDATE`,
			facilityUID,
		).Scan(&previousUserID); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`UPDATE facilities SET focal_person_id = NULL WHERE facility_uid = $1`,
			facilityUID,
		); err != nil {
			return err
		}

		if previousUserID != nil {
			return deactivateFacilityFocalRole(ctx, tx, *previousUserID, facilityUID)
		}
		return nil
	})
	if err != nil {
		return domain.Facility{}, err
	}
	return r.GetByUID(ctx, facilityUID)
}

func upsertFacilityFocalRole(ctx context.Context, tx pgx.Tx, userID, facilityUID string) error {
	var roleID string
	if err := tx.QueryRow(ctx,
		`SELECT id::text FROM roles WHERE code = $1`,
		focalPersonRoleCode,
	).Scan(&roleID); err != nil {
		return fmt.Errorf("lookup focal-person role: %w", err)
	}

	_, err := tx.Exec(ctx, `
		INSERT INTO user_roles (id, user_id, role_id, scope_type, scope_id, active, assigned_at)
		VALUES (gen_random_uuid(), $1, $2, 'FACILITY', $3, TRUE, now())
		ON CONFLICT (user_id, role_id, scope_type, scope_id)
		DO UPDATE SET active = TRUE, assigned_at = now()
	`, userID, roleID, facilityUID)
	return err
}

func deactivateFacilityFocalRole(ctx context.Context, tx pgx.Tx, userID, facilityUID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE user_roles
		SET active = FALSE
		WHERE user_id = $1
		  AND scope_type = 'FACILITY'
		  AND scope_id = $2
		  AND role_id = (SELECT id FROM roles WHERE code = $3)
	`, userID, facilityUID, focalPersonRoleCode)
	return err
}
