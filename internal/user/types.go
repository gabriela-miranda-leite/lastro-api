package user

import "time"

// ValidationError carries a user-facing message for a specific field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}

type UpdateInput struct {
	Name          *string  `json:"name"`
	MonthlyIncome *string  `json:"monthly_income"`
	SavingGoalPct *float64 `json:"saving_goal_pct"`
}

type DeleteInput struct {
	Password string `json:"password" binding:"required"`
}

type ChangePasswordInput struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required"`
}

type MeResponse struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Email         string           `json:"email"`
	MonthlyIncome *string          `json:"monthly_income"`
	SavingGoalPct *float64         `json:"saving_goal_pct"`
	CoupleID      *string          `json:"couple_id"`
	CreatedAt     time.Time        `json:"created_at"`
	Partner       *PartnerResponse `json:"partner"`
}

type PartnerResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}
