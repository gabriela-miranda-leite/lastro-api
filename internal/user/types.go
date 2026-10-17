package user

import "time"

type MeResponse struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	Email         string           `json:"email"`
	MonthlyIncome *float64         `json:"monthly_income"`
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
