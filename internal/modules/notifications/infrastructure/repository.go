package infrastructure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"dispatch/internal/modules/notifications/application"
	"dispatch/internal/modules/notifications/domain"
	platformdb "dispatch/internal/platform/db"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	db *pgxpool.Pool
}

func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

var _ application.Repository = (*Repository)(nil)

func (r *Repository) ListNotifications(ctx context.Context, userID string, p platformdb.Pagination) ([]domain.Notification, int64, error) {
	allowedSorts := map[string]string{
		"created_at": "n.created_at",
	}

	where := []string{"1=1"}
	args := make([]any, 0)
	argPos := 1

	if userID != "" {
		where = append(where, fmt.Sprintf("n.recipient_user_id = $%d", argPos))
		args = append(args, userID)
		argPos++
	}

	for key, value := range p.Filters {
		switch key {
		case "status":
			where = append(where, fmt.Sprintf("n.status = $%d", argPos))
			args = append(args, strings.ToUpper(value))
			argPos++
		case "channel":
			where = append(where, fmt.Sprintf("n.channel = $%d", argPos))
			args = append(args, strings.ToUpper(value))
			argPos++
		case "date_from":
			where = append(where, fmt.Sprintf("n.created_at >= $%d", argPos))
			args = append(args, value)
			argPos++
		case "date_to":
			where = append(where, fmt.Sprintf("n.created_at <= $%d", argPos))
			args = append(args, value)
			argPos++
		}
	}

	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(1) FROM notifications n `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	orderBy := platformdb.BuildOrderBy(p, allowedSorts)

	query := fmt.Sprintf(`
SELECT
	n.id,
	n.type,
	n.recipient_user_id,
	n.recipient_phone,
	n.recipient_email,
	n.title,
	n.body,
	n.channel,
	n.linked_entity_type,
	n.linked_entity_id,
	n.status,
	n.attempts,
	n.sent_at,
	n.read_at,
	n.created_at
FROM notifications n
%s
%s
LIMIT $%d OFFSET $%d`, whereSQL, orderBy, argPos, argPos+1)

	rows, err := r.db.Query(ctx, query, append(args, p.PageSize, p.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]domain.Notification, 0)
	for rows.Next() {
		var n domain.Notification
		if err := rows.Scan(
			&n.ID,
			&n.Type,
			&n.RecipientUserID,
			&n.RecipientPhone,
			&n.RecipientEmail,
			&n.Title,
			&n.Body,
			&n.Channel,
			&n.LinkedEntityType,
			&n.LinkedEntityID,
			&n.Status,
			&n.Attempts,
			&n.SentAt,
			&n.ReadAt,
			&n.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		items = append(items, n)
	}
	return items, total, rows.Err()
}

func (r *Repository) GetByID(ctx context.Context, id string) (domain.Notification, error) {
	const q = `
SELECT
	n.id,
	n.type,
	n.recipient_user_id,
	n.recipient_phone,
	n.recipient_email,
	n.title,
	n.body,
	n.channel,
	n.linked_entity_type,
	n.linked_entity_id,
	n.status,
	n.attempts,
	n.sent_at,
	n.read_at,
	n.created_at
FROM notifications n
WHERE n.id = $1`
	var n domain.Notification
	if err := r.db.QueryRow(ctx, q, id).Scan(
		&n.ID,
		&n.Type,
		&n.RecipientUserID,
		&n.RecipientPhone,
		&n.RecipientEmail,
		&n.Title,
		&n.Body,
		&n.Channel,
		&n.LinkedEntityType,
		&n.LinkedEntityID,
		&n.Status,
		&n.Attempts,
		&n.SentAt,
		&n.ReadAt,
		&n.CreatedAt,
	); err != nil {
		return domain.Notification{}, err
	}
	return n, nil
}

func (r *Repository) Create(ctx context.Context, in domain.Notification) (domain.Notification, error) {
	const q = `
INSERT INTO notifications (
	id, type, recipient_user_id, recipient_phone, recipient_email,
	title, body, channel, linked_entity_type, linked_entity_id,
	status, attempts, sent_at, read_at, created_at
) VALUES (
	$1,$2,$3,$4,$5,
	$6,$7,$8,$9,$10,
	$11,$12,$13,$14,$15
)
RETURNING created_at`
	if err := r.db.QueryRow(
		ctx,
		q,
		in.ID,
		in.Type,
		in.RecipientUserID,
		in.RecipientPhone,
		in.RecipientEmail,
		in.Title,
		in.Body,
		in.Channel,
		in.LinkedEntityType,
		in.LinkedEntityID,
		in.Status,
		in.Attempts,
		in.SentAt,
		in.ReadAt,
		in.CreatedAt,
	).Scan(&in.CreatedAt); err != nil {
		return domain.Notification{}, err
	}
	return in, nil
}

func (r *Repository) UpdateStatus(ctx context.Context, id string, status string) error {
	_, err := r.db.Exec(ctx, `UPDATE notifications SET status=$2 WHERE id=$1`, id, status)
	return err
}

func (r *Repository) UpdateStatusForUser(ctx context.Context, id string, userID string, status string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status=$3,
		    read_at = CASE WHEN $3 = 'READ' THEN now() ELSE read_at END
		WHERE id=$1 AND recipient_user_id=$2
	`, id, userID, status)
	return err
}

func (r *Repository) MarkSent(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status='SENT', sent_at=now()
		WHERE id=$1
	`, id)
	return err
}

func (r *Repository) MarkFailed(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status='FAILED'
		WHERE id=$1
	`, id)
	return err
}

func (r *Repository) IncrementAttempts(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET attempts = attempts + 1
		WHERE id=$1
	`, id)
	return err
}

func nullIfEmpty(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func nowPtr() *time.Time {
	t := time.Now().UTC()
	return &t
}
