package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// Repository handles data access for the audit domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) list(ctx context.Context, userID string, input ListInput) ([]Entry, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1`, userID,
	).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("audit.repository.list: count: %w", err)
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, action, metadata, created_at
		   FROM audit_log
		  WHERE user_id = $1
		  ORDER BY created_at DESC
		  LIMIT $2 OFFSET $3`,
		userID, input.Limit, input.Offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("audit.repository.list: query: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	entries := []Entry{}
	for rows.Next() {
		var e Entry
		var meta []byte
		if err := rows.Scan(&e.ID, &e.Action, &meta, &e.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("audit.repository.list: scan: %w", err)
		}
		if len(meta) > 0 {
			e.Metadata = json.RawMessage(meta)
		} else {
			e.Metadata = json.RawMessage("null")
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("audit.repository.list: rows: %w", err)
	}

	return entries, total, nil
}
