package infrastructure

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	bloodworkers "dispatch/internal/modules/blood/workers"
)

type BroadcastRecipientFinder struct {
	db *pgxpool.Pool
}

func NewBroadcastRecipientFinder(db *pgxpool.Pool) *BroadcastRecipientFinder {
	return &BroadcastRecipientFinder{db: db}
}

func (f *BroadcastRecipientFinder) FindBroadcastRecipients(ctx context.Context, bloodRequisitionID string) ([]bloodworkers.BroadcastRecipient, error) {
	rows, err := f.db.Query(ctx, `
		SELECT DISTINCT
			u.id,
			COALESCE(u.phone, ''),
			COALESCE(u.email, ''),
			COALESCE(u.first_name || ' ' || u.last_name, '')
		FROM blood_requisition_broadcasts brb
		LEFT JOIN users u ON u.id = brb.recipient_user_id
		WHERE brb.blood_requisition_id = $1
	`, bloodRequisitionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []bloodworkers.BroadcastRecipient
	for rows.Next() {
		var r bloodworkers.BroadcastRecipient
		if err := rows.Scan(&r.UserID, &r.Phone, &r.Email, &r.Name); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
