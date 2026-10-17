package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("not found")

// Repository handles data access for the user domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) findByID(ctx context.Context, userID string) (MeResponse, error) {
	const q = `
		SELECT
			u.id, u.name, u.email, u.monthly_income, u.saving_goal_pct, u.couple_id, u.created_at,
			p.id, p.name, p.email
		FROM users u
		LEFT JOIN users p
			ON p.id != u.id
			AND p.couple_id = u.couple_id
			AND u.couple_id IS NOT NULL
			AND p.deleted_at IS NULL
		WHERE u.id = $1
		  AND u.deleted_at IS NULL`

	var me MeResponse
	var partnerID, partnerName, partnerEmail sql.NullString

	err := r.db.QueryRowContext(ctx, q, userID).Scan(
		&me.ID, &me.Name, &me.Email,
		&me.MonthlyIncome, &me.SavingGoalPct, &me.CoupleID, &me.CreatedAt,
		&partnerID, &partnerName, &partnerEmail,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return MeResponse{}, ErrNotFound
		}
		return MeResponse{}, fmt.Errorf("user.repository.findByID: %w", err)
	}

	if partnerID.Valid {
		me.Partner = &PartnerResponse{
			ID:    partnerID.String,
			Name:  partnerName.String,
			Email: partnerEmail.String,
		}
	}

	return me, nil
}
