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

func (r *Repository) sameCouple(ctx context.Context, userIDA, userIDB string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM users u1
		   JOIN users u2 ON u2.couple_id = u1.couple_id
		  WHERE u1.id = $1 AND u2.id = $2
		    AND u1.couple_id IS NOT NULL`,
		userIDA, userIDB,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("transaction.repository.sameCouple: %w", err)
	}
	return count > 0, nil
}

func (r *Repository) list(ctx context.Context, userID string, input *ListInput) ([]Transaction, error) {
	args := []interface{}{userID, input.Year, input.Month}
	idx := 4

	filter := ""
	if input.CategoryID != nil {
		filter += fmt.Sprintf(" AND t.category_id = $%d", idx)
		args = append(args, *input.CategoryID)
		idx++
	}
	if input.Direction != nil {
		filter += fmt.Sprintf(" AND t.direction = $%d", idx)
		args = append(args, *input.Direction)
	}

	query := fmt.Sprintf(`
		SELECT t.id, t.client_id, t.user_id, t.category_id, t.amount::text,
		       t.direction, t.description, t.is_shared, t.date::text,
		       t.created_at, t.updated_at
		  FROM transactions t
		 WHERE t.user_id = $1
		   AND EXTRACT(YEAR  FROM t.date) = $2
		   AND EXTRACT(MONTH FROM t.date) = $3
		   AND t.deleted_at IS NULL
		%s
		 ORDER BY t.date DESC, t.created_at DESC`, filter)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("transaction.repository.list: %w", err)
	}
	defer rows.Close() //nolint:errcheck // read-only query

	var txs []Transaction
	for rows.Next() {
		var t Transaction
		var categoryID, description sql.NullString
		if err := rows.Scan(
			&t.ID, &t.ClientID, &t.UserID, &categoryID, &t.Amount,
			&t.Direction, &description, &t.IsShared, &t.Date,
			&t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("transaction.repository.list: %w", err)
		}
		if categoryID.Valid {
			t.CategoryID = &categoryID.String
		}
		if description.Valid {
			t.Description = &description.String
		}
		txs = append(txs, t)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("transaction.repository.list: %w", err)
	}
	return txs, nil
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
