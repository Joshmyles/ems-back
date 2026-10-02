package domain

import (
	"time"

	platformdb "dispatch/internal/platform/db"
)

// Entry is a stored audit-log record as read back from the trail.
type Entry struct {
	ID            string         `json:"id"`
	ActorUserID   *string        `json:"actor_user_id,omitempty"`
	ActorUsername string         `json:"actor_username"`
	ActorRoles    []string       `json:"actor_roles"`
	Action        string         `json:"action"`
	EntityType    string         `json:"entity_type"`
	EntityID      *string        `json:"entity_id,omitempty"`
	Status        string         `json:"status"`
	Description   string         `json:"description"`
	Before        any            `json:"before,omitempty"`
	After         any            `json:"after,omitempty"`
	IPAddress     string         `json:"ip_address,omitempty"`
	UserAgent     string         `json:"user_agent,omitempty"`
	RequestID     string         `json:"request_id,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
}

// ListParams filters the audit trail. Zero-value fields are ignored.
type ListParams struct {
	Pagination  platformdb.Pagination
	ActorUserID string
	Action      string
	EntityType  string
	EntityID    string
	Status      string
	From        *time.Time
	To          *time.Time
}
