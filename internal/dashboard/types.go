package dashboard

// TypeBreakdown holds the amount, percentage and status for one expense category type.
type TypeBreakdown struct {
	Amount    string `json:"amount"`
	Pct       string `json:"pct"`
	TargetPct string `json:"target_pct"`
	Status    string `json:"status"` // "under" | "met" | "over"
}

// ExpensesSummary aggregates all out-direction transactions for a month.
type ExpensesSummary struct {
	Total  string                    `json:"total"`
	ByType map[string]*TypeBreakdown `json:"by_type"`
}

// SavingsSummary holds the monthly savings goal progress.
type SavingsSummary struct {
	Goal   string `json:"goal"`
	Saved  string `json:"saved"`
	Status string `json:"status"`
}

// CoupleSummary holds the consolidated financials for both partners.
type CoupleSummary struct {
	Income   string           `json:"income"`
	Expenses *ExpensesSummary `json:"expenses"`
}

// Response is the full payload for GET /dashboard/:year/:month.
type Response struct {
	Year     int              `json:"year"`
	Month    int              `json:"month"`
	Income   string           `json:"income"`
	Expenses *ExpensesSummary `json:"expenses"`
	Savings  *SavingsSummary  `json:"savings"`
	Couple   *CoupleSummary   `json:"couple"`
}
