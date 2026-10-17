package transaction

import "time"

var (
	ErrDuplicate           = errorSentinel("duplicate client_id")
	ErrSharedWithoutCouple = errorSentinel("is_shared requires couple")
	ErrCategoryForbidden   = errorSentinel("category belongs to another user")
	ErrNotFound            = errorSentinel("not found")
	ErrForbidden           = errorSentinel("forbidden")
)

type errorSentinel string

func (e errorSentinel) Error() string { return string(e) }

// CreateInput is the request body for POST /transactions.
type CreateInput struct {
	ClientID    string  `json:"client_id"    binding:"required,uuid"`
	Amount      float64 `json:"amount"       binding:"required"`
	Direction   string  `json:"direction"    binding:"required"`
	CategoryID  *string `json:"category_id"`
	Description *string `json:"description"`
	IsShared    bool    `json:"is_shared"`
	Date        string  `json:"date"         binding:"required"`
}

// Transaction is the domain representation returned to clients.
type Transaction struct {
	ID          string    `json:"id"`
	ClientID    string    `json:"client_id"`
	UserID      string    `json:"user_id"`
	CategoryID  *string   `json:"category_id"`
	Amount      string    `json:"amount"` // NUMERIC returned as string to avoid float precision
	Direction   string    `json:"direction"`
	Description *string   `json:"description"`
	IsShared    bool      `json:"is_shared"`
	Date        string    `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ListInput holds the query params for GET /transactions.
type ListInput struct {
	Year       int
	Month      int
	CategoryID *string
	Direction  *string
	UserID     *string // if set, list transactions of this user (partner check required)
}

// ValidationError carries a field-level validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
