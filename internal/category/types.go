package category

import "time"

const (
	TypeNeeds   = "NEEDS"
	TypeWants   = "WANTS"
	TypeSavings = "SAVINGS"
)

var (
	ErrNotFound  = errorSentinel("not found")
	ErrForbidden = errorSentinel("forbidden")
	ErrConflict  = errorSentinel("conflict")
)

type errorSentinel string

func (e errorSentinel) Error() string { return string(e) }

// Category is the domain model for a user-defined transaction category.
type Category struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Icon      *string   `json:"icon,omitempty"`
	Color     *string   `json:"color,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateInput is the request body for POST /categories.
type CreateInput struct {
	Name  string  `json:"name"  binding:"required"`
	Type  string  `json:"type"  binding:"required"`
	Icon  *string `json:"icon"`
	Color *string `json:"color"`
}

// UpdateInput is the request body for PUT /categories/:id.
type UpdateInput struct {
	Name  *string `json:"name"`
	Type  *string `json:"type"`
	Icon  *string `json:"icon"`
	Color *string `json:"color"`
}
