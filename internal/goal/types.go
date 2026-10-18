package goal

import "time"

var (
	ErrConflict = errorSentinel("conflict")
	ErrNotFound = errorSentinel("not found")
	ErrForbidden = errorSentinel("forbidden")
)

type errorSentinel string

func (e errorSentinel) Error() string { return string(e) }

// Goal is the domain representation of a monthly savings goal.
type Goal struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Year        int       `json:"year"`
	Month       int       `json:"month"`
	GoalAmount  string    `json:"goal_amount"`  // NUMERIC returned as string
	SavedAmount string    `json:"saved_amount"` // NUMERIC returned as string
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateInput is the request body for POST /goals.
type CreateInput struct {
	Year       int     `json:"year"         binding:"required"`
	Month      int     `json:"month"        binding:"required"`
	GoalAmount float64 `json:"goal_amount"  binding:"required"`
}

// GoalWithStatus wraps Goal with a calculated status field.
type GoalWithStatus struct {
	Goal
	Status string `json:"status"` // "under" | "met" | "over"
}

// ListInput holds optional filters for GET /goals.
type ListInput struct {
	Year  *int
	Month *int
}

// UpdateInput is the request body for PUT /goals/:id.
type UpdateInput struct {
	SavedAmount *float64 `json:"saved_amount" binding:"required"`
}

// ValidationError carries a field-level validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
