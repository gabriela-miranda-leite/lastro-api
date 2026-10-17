package recurring

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Repository handles data access for the recurring domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func scanRecurring(row interface {
	Scan(...interface{}) error
}) (Recurring, error) {
	var r Recurring
	var categoryID sql.NullString
	err := row.Scan(
		&r.ID, &r.UserID, &categoryID, &r.Name, &r.Amount,
		&r.DueDay, &r.Active, &r.CreatedAt, &r.UpdatedAt,
	)
	if err != nil {
		return Recurring{}, fmt.Errorf("recurring.repository: %w", err)
	}
	if categoryID.Valid {
		r.CategoryID = &categoryID.String
	}
	return r, nil
}

func (r *Repository) list(ctx context.Context, userID string) ([]Recurring, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, category_id, name, amount::text,
		        due_day, active, created_at, updated_at
		   FROM recurring_expenses
		  WHERE user_id = $1
		  ORDER BY due_day ASC, created_at ASC`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("recurring.repository.list: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	var result []Recurring
	for rows.Next() {
		rec, err := scanRecurring(rows)
		if err != nil {
			return nil, fmt.Errorf("recurring.repository.list: %w", err)
		}
		result = append(result, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("recurring.repository.list: %w", err)
	}
	return result, nil
}

func (r *Repository) findByID(ctx context.Context, id string) (Recurring, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, category_id, name, amount::text,
		        due_day, active, created_at, updated_at
		   FROM recurring_expenses
		  WHERE id = $1`,
		id,
	)
	rec, err := scanRecurring(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Recurring{}, ErrNotFound
	}
	if err != nil {
		return Recurring{}, fmt.Errorf("recurring.repository.findByID: %w", err)
	}
	return rec, nil
}

func (r *Repository) insert(ctx context.Context, userID string, input *CreateInput) (Recurring, error) {
	active := true
	if input.Active != nil {
		active = *input.Active
	}
	var catArg interface{}
	if input.CategoryID != nil {
		catArg = *input.CategoryID
	}
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO recurring_expenses (user_id, category_id, name, amount, due_day, active)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, user_id, category_id, name, amount::text,
		           due_day, active, created_at, updated_at`,
		userID, catArg, input.Name, input.Amount, input.DueDay, active,
	)
	rec, err := scanRecurring(row)
	if err != nil {
		return Recurring{}, fmt.Errorf("recurring.repository.insert: %w", err)
	}
	return rec, nil
}

func (r *Repository) update(ctx context.Context, id string, input *UpdateInput) (Recurring, error) {
	sets := []string{"updated_at = now()"}
	args := []interface{}{}
	idx := 1

	if input.Name != nil {
		sets = append(sets, fmt.Sprintf("name = $%d", idx))
		args = append(args, *input.Name)
		idx++
	}
	if input.Amount != nil {
		sets = append(sets, fmt.Sprintf("amount = $%d", idx))
		args = append(args, *input.Amount)
		idx++
	}
	if input.DueDay != nil {
		sets = append(sets, fmt.Sprintf("due_day = $%d", idx))
		args = append(args, *input.DueDay)
		idx++
	}
	if input.CategoryID != nil {
		sets = append(sets, fmt.Sprintf("category_id = $%d", idx))
		args = append(args, *input.CategoryID)
		idx++
	}
	if input.Active != nil {
		sets = append(sets, fmt.Sprintf("active = $%d", idx))
		args = append(args, *input.Active)
		idx++
	}

	args = append(args, id)
	query := fmt.Sprintf(
		`UPDATE recurring_expenses SET %s WHERE id = $%d
		 RETURNING id, user_id, category_id, name, amount::text,
		           due_day, active, created_at, updated_at`,
		strings.Join(sets, ", "), idx,
	)
	row := r.db.QueryRowContext(ctx, query, args...)
	rec, err := scanRecurring(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Recurring{}, ErrNotFound
	}
	if err != nil {
		return Recurring{}, fmt.Errorf("recurring.repository.update: %w", err)
	}
	return rec, nil
}

func (r *Repository) delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx,
		`DELETE FROM recurring_expenses WHERE id = $1`,
		id,
	)
	if err != nil {
		return fmt.Errorf("recurring.repository.delete: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("recurring.repository.delete: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) upsertOverride(ctx context.Context, recurringID string, input *OverrideInput) (RecurringOverride, error) {
	var amountArg interface{}
	if input.AmountOverride != nil {
		amountArg = *input.AmountOverride
	}
	row := r.db.QueryRowContext(ctx,
		`INSERT INTO recurring_overrides (recurring_id, year, month, amount_override, active)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (recurring_id, year, month) DO UPDATE
		   SET amount_override = EXCLUDED.amount_override,
		       active          = EXCLUDED.active
		 RETURNING id, recurring_id, year, month, amount_override::text, active, created_at`,
		recurringID, input.Year, input.Month, amountArg, input.Active,
	)
	var o RecurringOverride
	var amountOverride sql.NullString
	err := row.Scan(&o.ID, &o.RecurringID, &o.Year, &o.Month, &amountOverride, &o.Active, &o.CreatedAt)
	if err != nil {
		return RecurringOverride{}, fmt.Errorf("recurring.repository.upsertOverride: %w", err)
	}
	if amountOverride.Valid {
		o.AmountOverride = &amountOverride.String
	}
	return o, nil
}
