package couple

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// Repository handles data access for the couple domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) isInCouple(ctx context.Context, userID string) (bool, error) {
	var coupleID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT couple_id FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&coupleID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("couple.repository.isInCouple: %w", err)
	}
	return coupleID.Valid, nil
}

func (r *Repository) invalidatePreviousInvites(ctx context.Context, userID string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE couple_invites
		    SET used_at = now()
		  WHERE inviter_id = $1
		    AND used_at IS NULL
		    AND expires_at > now()`,
		userID,
	)
	if err != nil {
		return fmt.Errorf("couple.repository.invalidatePreviousInvites: %w", err)
	}
	return nil
}

func (r *Repository) createInvite(ctx context.Context, userID, code string) (InviteResponse, error) {
	expiresAt := time.Now().Add(24 * time.Hour)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO couple_invites (inviter_id, code, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, code, expiresAt,
	)
	if err != nil {
		return InviteResponse{}, fmt.Errorf("couple.repository.createInvite: %w", err)
	}
	return InviteResponse{Code: code, ExpiresAt: expiresAt}, nil
}

func (r *Repository) codeExists(ctx context.Context, code string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM couple_invites WHERE code = $1)`,
		code,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("couple.repository.codeExists: %w", err)
	}
	return exists, nil
}

func (r *Repository) findInviteByCode(ctx context.Context, code string) (InviteRecord, error) {
	var rec InviteRecord
	var usedAt sql.NullTime
	err := r.db.QueryRowContext(ctx,
		`SELECT id, inviter_id, expires_at, used_at
		   FROM couple_invites
		  WHERE code = $1`,
		code,
	).Scan(&rec.ID, &rec.InviterID, &rec.ExpiresAt, &usedAt)
	if err == sql.ErrNoRows {
		return InviteRecord{}, ErrInviteNotFound
	}
	if err != nil {
		return InviteRecord{}, fmt.Errorf("couple.repository.findInviteByCode: %w", err)
	}
	if usedAt.Valid {
		t := usedAt.Time
		rec.UsedAt = &t
	}
	return rec, nil
}

// joinCouple runs a transaction: creates a couple, links both users, marks invite used.
func (r *Repository) joinCouple(ctx context.Context, inviteID, inviterID, joinerID string) (JoinResponse, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return JoinResponse{}, fmt.Errorf("couple.repository.joinCouple: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback is a no-op after commit

	var coupleID string
	if err := tx.QueryRowContext(ctx,
		`INSERT INTO couples DEFAULT VALUES RETURNING id`,
	).Scan(&coupleID); err != nil {
		return JoinResponse{}, fmt.Errorf("couple.repository.joinCouple: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE users SET couple_id = $1 WHERE id = ANY($2::uuid[]) AND deleted_at IS NULL`,
		coupleID, fmt.Sprintf("{%s,%s}", inviterID, joinerID),
	); err != nil {
		return JoinResponse{}, fmt.Errorf("couple.repository.joinCouple: %w", err)
	}

	if _, err := tx.ExecContext(ctx,
		`UPDATE couple_invites SET used_at = now() WHERE id = $1`,
		inviteID,
	); err != nil {
		return JoinResponse{}, fmt.Errorf("couple.repository.joinCouple: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return JoinResponse{}, fmt.Errorf("couple.repository.joinCouple: %w", err)
	}

	return JoinResponse{CoupleID: coupleID}, nil
}
