package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

var ErrNotFound = errors.New("not found")
var ErrNoFieldsToUpdate = errors.New("no fields to update")

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

func (r *Repository) update(ctx context.Context, userID string, input UpdateInput) error {
	setClauses := make([]string, 0, 3)
	args := make([]any, 0, 4)
	idx := 1

	if input.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", idx))
		args = append(args, *input.Name)
		idx++
	}
	if input.MonthlyIncome != nil {
		setClauses = append(setClauses, fmt.Sprintf("monthly_income = $%d", idx))
		args = append(args, *input.MonthlyIncome)
		idx++
	}
	if input.SavingGoalPct != nil {
		setClauses = append(setClauses, fmt.Sprintf("saving_goal_pct = $%d", idx))
		args = append(args, *input.SavingGoalPct)
		idx++
	}

	if len(setClauses) == 0 {
		return ErrNoFieldsToUpdate
	}

	args = append(args, userID)
	q := fmt.Sprintf(
		"UPDATE users SET %s WHERE id = $%d AND deleted_at IS NULL",
		strings.Join(setClauses, ", "), idx,
	)

	res, err := r.db.ExecContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("user.repository.update: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("user.repository.update: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
