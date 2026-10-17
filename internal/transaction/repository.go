package transaction

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository handles data access for the transaction domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) findByClientID(ctx context.Context, clientID string) (Transaction, error) {
	var t Transaction
	var categoryID sql.NullString
	var description sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, client_id, user_id, category_id, amount::text, direction,
		        description, is_shared, date::text, created_at, updated_at
		   FROM transactions
		  WHERE client_id = $1 AND deleted_at IS NULL`,
		clientID,
	).Scan(
		&t.ID, &t.ClientID, &t.UserID, &categoryID, &t.Amount,
		&t.Direction, &description, &t.IsShared, &t.Date, &t.CreatedAt, &t.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Transaction{}, ErrNotFound
	}
	if err != nil {
		return Transaction{}, fmt.Errorf("transaction.repository.findByClientID: %w", err)
	}
	if categoryID.Valid {
		t.CategoryID = &categoryID.String
	}
	if description.Valid {
		t.Description = &description.String
	}
	return t, nil
}

// categoryOwner returns (ownerID, true, nil) for user-owned categories,
// ("", false, nil) for system categories, or ("", false, err) on failure.
func (r *Repository) categoryOwner(ctx context.Context, categoryID string) (owner string, owned bool, err error) {
	var nullOwner sql.NullString
	err = r.db.QueryRowContext(ctx,
		`SELECT user_id FROM categories WHERE id = $1`,
		categoryID,
	).Scan(&nullOwner)
	if err == sql.ErrNoRows {
		return "", false, ErrNotFound
	}
	if err != nil {
		return "", false, fmt.Errorf("transaction.repository.categoryOwner: %w", err)
	}
	if nullOwner.Valid {
		return nullOwner.String, true, nil
	}
	return "", false, nil // system category
}

func (r *Repository) userHasCouple(ctx context.Context, userID string) (bool, error) {
	var coupleID sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT couple_id FROM users WHERE id = $1 AND deleted_at IS NULL`,
		userID,
	).Scan(&coupleID)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("transaction.repository.userHasCouple: %w", err)
	}
	return coupleID.Valid, nil
}

func (r *Repository) insert(ctx context.Context, userID string, input *CreateInput) (Transaction, error) {
	var t Transaction
	var categoryID sql.NullString
	var description sql.NullString

	var catArg interface{}
	if input.CategoryID != nil {
		catArg = *input.CategoryID
	}
	var descArg interface{}
	if input.Description != nil {
		descArg = *input.Description
	}

	err := r.db.QueryRowContext(ctx,
		`INSERT INTO transactions
		        (client_id, user_id, category_id, amount, direction, description, is_shared, date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::date)
		 RETURNING id, client_id, user_id, category_id, amount::text, direction,
		           description, is_shared, date::text, created_at, updated_at`,
		input.ClientID, userID, catArg, input.Amount, input.Direction, descArg, input.IsShared, input.Date,
	).Scan(
		&t.ID, &t.ClientID, &t.UserID, &categoryID, &t.Amount,
		&t.Direction, &description, &t.IsShared, &t.Date, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return Transaction{}, fmt.Errorf("transaction.repository.insert: %w", err)
	}
	if categoryID.Valid {
		t.CategoryID = &categoryID.String
	}
	if description.Valid {
		t.Description = &description.String
	}
	return t, nil
}
