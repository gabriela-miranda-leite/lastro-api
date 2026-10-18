package dashboard

import (
	"context"
	"fmt"
	"math"
	"strconv"
)

// targetPcts maps category type to its 50/30/20 target percentage.
var targetPcts = map[string]string{
	"necessity":  "50",
	"leisure":    "30",
	"investment": "20",
	"saving":     "20",
}

// allTypes is the fixed order we always include in by_type, even with zero amounts.
var allTypes = []string{"necessity", "leisure", "investment", "saving"}

// Service contains business logic for the dashboard domain.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Get builds the dashboard response for the given user, year and month.
func (s *Service) Get(ctx context.Context, userID string, year, month int) (Response, error) {
	userInfo, err := s.repo.FetchUserInfo(ctx, userID)
	if err != nil {
		return Response{}, fmt.Errorf("dashboard.service.Get: %w", err)
	}

	income, expenses, savings, err := s.fetchPersonal(ctx, userID, year, month)
	if err != nil {
		return Response{}, fmt.Errorf("dashboard.service.Get: %w", err)
	}

	resp := Response{
		Year:     year,
		Month:    month,
		Income:   income,
		Expenses: expenses,
		Savings:  savings,
	}

	if userInfo.CoupleID.Valid {
		partnerID, err := s.repo.FetchPartnerID(ctx, userInfo.CoupleID.String, userID)
		if err != nil {
			return Response{}, fmt.Errorf("dashboard.service.Get: %w", err)
		}
		if partnerID != "" {
			resp.Couple, err = s.buildCouple(ctx, userID, partnerID, year, month)
			if err != nil {
				return Response{}, fmt.Errorf("dashboard.service.Get: %w", err)
			}
		}
	}

	return resp, nil
}

func (s *Service) fetchPersonal(ctx context.Context, userID string, year, month int) (string, *ExpensesSummary, *SavingsSummary, error) {
	income, err := s.repo.FetchIncome(ctx, userID, year, month)
	if err != nil {
		return "", nil, nil, err
	}

	typeAmounts, err := s.repo.FetchExpensesByType(ctx, userID, year, month)
	if err != nil {
		return "", nil, nil, err
	}

	expenses := buildExpenses(typeAmounts, income)

	savingsRow, found, err := s.repo.FetchSavings(ctx, userID, year, month)
	if err != nil {
		return "", nil, nil, err
	}
	var savings *SavingsSummary
	if found {
		savings = buildSavings(&savingsRow)
	}

	return income, expenses, savings, nil
}

func (s *Service) buildCouple(ctx context.Context, userID, partnerID string, year, month int) (*CoupleSummary, error) {
	userIncome, err := s.repo.FetchIncome(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}
	partnerIncome, err := s.repo.FetchIncome(ctx, partnerID, year, month)
	if err != nil {
		return nil, err
	}

	userTypes, err := s.repo.FetchExpensesByType(ctx, userID, year, month)
	if err != nil {
		return nil, err
	}
	partnerTypes, err := s.repo.FetchExpensesByType(ctx, partnerID, year, month)
	if err != nil {
		return nil, err
	}

	combinedIncome := addAmounts(userIncome, partnerIncome)
	combined := mergeTypeAmounts(userTypes, partnerTypes)
	expenses := buildExpenses(combined, combinedIncome)

	return &CoupleSummary{
		Income:   combinedIncome,
		Expenses: expenses,
	}, nil
}

// buildExpenses constructs ExpensesSummary from raw type→amount rows.
func buildExpenses(typeAmounts []TypeAmount, income string) *ExpensesSummary {
	incomeF := mustParseFloat(income)

	amountMap := map[string]float64{}
	for _, ta := range typeAmounts {
		amountMap[ta.Type] = mustParseFloat(ta.Amount)
	}

	var totalF float64
	for _, v := range amountMap {
		totalF += v
	}

	byType := make(map[string]*TypeBreakdown, len(allTypes))
	for _, t := range allTypes {
		amt := amountMap[t]
		byType[t] = &TypeBreakdown{
			Amount:    formatAmount(amt),
			Pct:       formatPct(pct(amt, incomeF)),
			TargetPct: targetPcts[t],
			Status:    calcStatus(amt, targetPct(t, incomeF)),
		}
	}

	return &ExpensesSummary{
		Total:  formatAmount(totalF),
		ByType: byType,
	}
}

// buildSavings constructs SavingsSummary from a SavingsRow.
func buildSavings(s *SavingsRow) *SavingsSummary {
	saved := mustParseFloat(s.SavedAmount)
	goal := mustParseFloat(s.GoalAmount)
	return &SavingsSummary{
		Goal:   s.GoalAmount,
		Saved:  s.SavedAmount,
		Status: calcStatusF(saved, goal),
	}
}

// mergeTypeAmounts sums two slices of TypeAmount into one.
func mergeTypeAmounts(a, b []TypeAmount) []TypeAmount {
	m := map[string]float64{}
	for _, ta := range a {
		m[ta.Type] += mustParseFloat(ta.Amount)
	}
	for _, ta := range b {
		m[ta.Type] += mustParseFloat(ta.Amount)
	}
	result := make([]TypeAmount, 0, len(m))
	for k, v := range m {
		result = append(result, TypeAmount{Type: k, Amount: formatAmount(v)})
	}
	return result
}

func addAmounts(a, b string) string {
	return formatAmount(mustParseFloat(a) + mustParseFloat(b))
}

// targetPct returns the absolute amount that corresponds to the target percentage of income.
func targetPct(categoryType string, income float64) float64 {
	tgt, ok := map[string]float64{
		"necessity":  50,
		"leisure":    30,
		"investment": 20,
		"saving":     20,
	}[categoryType]
	if !ok {
		return 0
	}
	return income * tgt / 100
}

func pct(amount, income float64) float64 {
	if income == 0 {
		return 0
	}
	return amount / income * 100
}

func calcStatus(actual, target float64) string {
	switch {
	case actual < target:
		return "under"
	case actual == target:
		return "met"
	default:
		return "over"
	}
}

func calcStatusF(saved, goal float64) string {
	return calcStatus(saved, goal)
}

func mustParseFloat(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func formatAmount(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', 2, 64)
}

func formatPct(v float64) string {
	return strconv.FormatFloat(math.Round(v*100)/100, 'f', 2, 64)
}
