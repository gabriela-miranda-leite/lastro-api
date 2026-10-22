package dashboard

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Repository handles data access for the dashboard domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// UserInfo holds the fields we need from the users table.
type UserInfo struct {
	CoupleID sql.NullString
}

// FetchUserInfo returns couple_id for the given user.
func (r *Repository) FetchUserInfo(ctx context.Context, userID string) (UserInfo, error) {
	var u UserInfo
	err := r.db.QueryRowContext(ctx,
		`SELECT couple_id FROM users WHERE id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&u.CoupleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserInfo{}, fmt.Errorf("dashboard.repository.FetchUserInfo: user not found")
		}
		return UserInfo{}, fmt.Errorf("dashboard.repository.FetchUserInfo: %w", err)
	}
	return u, nil
}

// FetchPartnerID returns the other member of a couple, or "" if none.
func (r *Repository) FetchPartnerID(ctx context.Context, coupleID, userID string) (string, error) {
	var partnerID string
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM users WHERE couple_id = $1 AND id != $2 AND deleted_at IS NULL`,
		coupleID, userID,
	).Scan(&partnerID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("dashboard.repository.FetchPartnerID: %w", err)
	}
	return partnerID, nil
}

// FetchIncome returns the total income for the user in the given month as a NUMERIC string.
func (r *Repository) FetchIncome(ctx context.Context, userID string, year, month int) (string, error) {
	var total string
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(amount), 0)::numeric(12,2)::text
		   FROM transactions
		  WHERE user_id = $1
		    AND direction = 'INCOME'
		    AND EXTRACT(YEAR  FROM date) = $2
		    AND EXTRACT(MONTH FROM date) = $3
		    AND deleted_at IS NULL`,
		userID, year, month,
	).Scan(&total)
	if err != nil {
		return "", fmt.Errorf("dashboard.repository.FetchIncome: %w", err)
	}
	return total, nil
}

// TypeAmount holds a category type with its summed amount.
type TypeAmount struct {
	Type   string
	Amount string
}

// FetchExpensesByType returns summed out-direction amounts grouped by category type.
func (r *Repository) FetchExpensesByType(ctx context.Context, userID string, year, month int) ([]TypeAmount, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT c.type, COALESCE(SUM(t.amount), 0)::text
		   FROM transactions t
		   JOIN categories c ON c.id = t.category_id
		  WHERE t.user_id = $1
		    AND t.direction = 'EXPENSE'
		    AND EXTRACT(YEAR  FROM t.date) = $2
		    AND EXTRACT(MONTH FROM t.date) = $3
		    AND t.deleted_at IS NULL
		  GROUP BY c.type`,
		userID, year, month,
	)
	if err != nil {
		return nil, fmt.Errorf("dashboard.repository.FetchExpensesByType: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	var result []TypeAmount
	for rows.Next() {
		var ta TypeAmount
		if err := rows.Scan(&ta.Type, &ta.Amount); err != nil {
			return nil, fmt.Errorf("dashboard.repository.FetchExpensesByType: %w", err)
		}
		result = append(result, ta)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("dashboard.repository.FetchExpensesByType: %w", err)
	}
	return result, nil
}

// SavingsRow holds the monthly savings goal data for a user.
type SavingsRow struct {
	GoalAmount  string
	SavedAmount string
}

// FetchSavings returns the monthly savings goal for the user.
// found is false when no goal exists for the month.
func (r *Repository) FetchSavings(ctx context.Context, userID string, year, month int) (s SavingsRow, found bool, err error) {
	scanErr := r.db.QueryRowContext(ctx,
		`SELECT goal_amount::text, saved_amount::text
		   FROM monthly_savings
		  WHERE user_id = $1 AND year = $2 AND month = $3`,
		userID, year, month,
	).Scan(&s.GoalAmount, &s.SavedAmount)
	if scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return SavingsRow{}, false, nil
		}
		return SavingsRow{}, false, fmt.Errorf("dashboard.repository.FetchSavings: %w", scanErr)
	}
	return s, true, nil
}
