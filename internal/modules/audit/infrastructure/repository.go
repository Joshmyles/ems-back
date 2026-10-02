package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	auditdomain "dispatch/internal/modules/audit/domain"
	"dispatch/internal/shared/types"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Insert writes a single audit entry. Callers pass a types.AuditEntry (the
// shared, dependency-free shape); the repository owns the mapping to columns.
func (r *Repository) Insert(ctx context.Context, e types.AuditEntry) error {
	status := e.Status
	if status == "" {
		status = types.AuditStatusSuccess
	}
	roles := e.ActorRoles
	if roles == nil {
		roles = []string{}
	}

	_, err := r.db.Exec(ctx, `
		INSERT INTO audit_logs (
			actor_user_id, actor_username, actor_roles, action, entity_type,
			entity_id, status, description, before_json, after_json,
			ip_address, user_agent, request_id, metadata
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14
		)`,
		nilIfEmpty(e.ActorUserID),
		e.ActorUsername,
		roles,
		e.Action,
		e.EntityType,
		nilIfEmpty(e.EntityID),
		status,
		e.Description,
		toJSON(e.Before),
		toJSON(e.After),
		nilIfEmpty(e.IPAddress),
		e.UserAgent,
		e.RequestID,
		toJSON(e.Metadata),
	)
	return err
}

func (r *Repository) List(ctx context.Context, params auditdomain.ListParams) ([]auditdomain.Entry, int64, error) {
	conds := []string{"1=1"}
	args := []any{}
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}

	if params.ActorUserID != "" {
		add("actor_user_id = $%d", params.ActorUserID)
	}
	if params.Action != "" {
		add("action = $%d", params.Action)
	}
	if params.EntityType != "" {
		add("entity_type = $%d", params.EntityType)
	}
	if params.EntityID != "" {
		add("entity_id = $%d", params.EntityID)
	}
	if params.Status != "" {
		add("status = $%d", strings.ToUpper(params.Status))
	}
	if params.From != nil {
		add("created_at >= $%d", *params.From)
	}
	if params.To != nil {
		add("created_at <= $%d", *params.To)
	}
	if s := strings.TrimSpace(params.Pagination.Search); s != "" {
		args = append(args, "%"+s+"%")
		conds = append(conds, fmt.Sprintf(
			"(actor_username ILIKE $%d OR description ILIKE $%d OR action ILIKE $%d)",
			len(args), len(args), len(args)))
	}

	where := strings.Join(conds, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	args = append(args, params.Pagination.PageSize, params.Pagination.Offset)
	query := fmt.Sprintf(`
		SELECT id, actor_user_id::text, COALESCE(actor_username,''), actor_roles, action, entity_type,
		       entity_id::text, status, COALESCE(description,''), before_json, after_json,
		       COALESCE(host(ip_address),''), COALESCE(user_agent,''), COALESCE(request_id,''), metadata, created_at
		FROM audit_logs
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args))

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []auditdomain.Entry
	for rows.Next() {
		var (
			e                               auditdomain.Entry
			beforeB, afterB, metaB          []byte
			actorID, entityID               *string
		)
		if err := rows.Scan(
			&e.ID, &actorID, &e.ActorUsername, &e.ActorRoles, &e.Action, &e.EntityType,
			&entityID, &e.Status, &e.Description, &beforeB, &afterB,
			&e.IPAddress, &e.UserAgent, &e.RequestID, &metaB, &e.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		e.ActorUserID = actorID
		e.EntityID = entityID
		e.Before = fromJSON(beforeB)
		e.After = fromJSON(afterB)
		if m := fromJSON(metaB); m != nil {
			if mm, ok := m.(map[string]any); ok {
				e.Metadata = mm
			}
		}
		if e.ActorRoles == nil {
			e.ActorRoles = []string{}
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func nilIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

func toJSON(v any) any {
	if v == nil {
		return nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	// An explicit JSON null carries no information; store SQL NULL instead.
	if string(b) == "null" {
		return nil
	}
	return b
}

func fromJSON(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return nil
	}
	return v
}
