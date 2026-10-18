package category

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository handles data access for the category domain.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new Repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) ListByUser(ctx context.Context, userID string) ([]Category, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, user_id, name, type, icon, created_at
		   FROM categories
		  WHERE user_id = $1
		  ORDER BY name`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("category.repository.ListByUser: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var cats []Category
	for rows.Next() {
		var c Category
		var icon sql.NullString
		if err := rows.Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &icon, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("category.repository.ListByUser scan: %w", err)
		}
		if icon.Valid {
			c.Icon = &icon.String
		}
		cats = append(cats, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("category.repository.ListByUser rows: %w", err)
	}
	return cats, nil
}

func (r *Repository) Insert(ctx context.Context, userID string, input CreateInput) (Category, error) {
	var c Category
	var icon sql.NullString
	err := r.db.QueryRowContext(ctx,
		`INSERT INTO categories (user_id, name, type, icon)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, user_id, name, type, icon, created_at`,
		userID, input.Name, input.Type, input.Icon,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &icon, &c.CreatedAt)
	if err != nil {
		return Category{}, fmt.Errorf("category.repository.Insert: %w", err)
	}
	if icon.Valid {
		c.Icon = &icon.String
	}
	return c, nil
}

func (r *Repository) FindByID(ctx context.Context, id string) (Category, error) {
	var c Category
	var icon sql.NullString
	err := r.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, type, icon, created_at
		   FROM categories
		  WHERE id = $1`,
		id,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &icon, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, fmt.Errorf("category.repository.FindByID: %w", err)
	}
	if icon.Valid {
		c.Icon = &icon.String
	}
	return c, nil
}

func (r *Repository) Update(ctx context.Context, id string, input UpdateInput) (Category, error) {
	var c Category
	var icon sql.NullString
	err := r.db.QueryRowContext(ctx,
		`UPDATE categories
		    SET name = COALESCE($2, name),
		        type = COALESCE($3, type),
		        icon = COALESCE($4, icon)
		  WHERE id = $1
		  RETURNING id, user_id, name, type, icon, created_at`,
		id, input.Name, input.Type, input.Icon,
	).Scan(&c.ID, &c.UserID, &c.Name, &c.Type, &icon, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return Category{}, ErrNotFound
	}
	if err != nil {
		return Category{}, fmt.Errorf("category.repository.Update: %w", err)
	}
	if icon.Valid {
		c.Icon = &icon.String
	}
	return c, nil
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("category.repository.Delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
