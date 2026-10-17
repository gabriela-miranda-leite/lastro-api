package recurring

import "time"

var (
	ErrNotFound  = errorSentinel("not found")
	ErrForbidden = errorSentinel("forbidden")
)

type errorSentinel string

func (e errorSentinel) Error() string { return string(e) }

// Recurring is the domain representation of a recurring expense.
type Recurring struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	CategoryID *string   `json:"category_id"`
	Name       string    `json:"name"`
	Amount     string    `json:"amount"` // NUMERIC returned as string
	DueDay     int       `json:"due_day"`
	Active     bool      `json:"active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreateInput is the request body for POST /recurring.
type CreateInput struct {
	Name       string  `json:"name"        binding:"required"`
	Amount     float64 `json:"amount"      binding:"required"`
	DueDay     int     `json:"due_day"     binding:"required"`
	CategoryID *string `json:"category_id"`
	Active     *bool   `json:"active"` // defaults to true if nil
}

// UpdateInput holds optional fields for PUT /recurring/:id.
type UpdateInput struct {
	Name       *string  `json:"name"`
	Amount     *float64 `json:"amount"`
	DueDay     *int     `json:"due_day"`
	CategoryID *string  `json:"category_id"`
	Active     *bool    `json:"active"`
}

// ValidationError carries a field-level validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
