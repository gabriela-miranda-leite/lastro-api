package dashboard

// Unit tests for the dashboard service layer.
// All tests run without a database — the repository is mocked via dashboardRepo.

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock repo ---

type mockRepo struct {
	userInfo       UserInfo
	userInfoErr    error
	partnerID      string
	partnerIDErr   error
	incomeByUser   map[string]string
	incomeErr      error
	expensesByUser map[string][]TypeAmount
	expensesErr    error
	savings        SavingsRow
	savingsFound   bool
	savingsErr     error
}

func (m *mockRepo) FetchUserInfo(_ context.Context, _ string) (UserInfo, error) {
	return m.userInfo, m.userInfoErr
}

func (m *mockRepo) FetchPartnerID(_ context.Context, _, _ string) (string, error) {
	return m.partnerID, m.partnerIDErr
}

func (m *mockRepo) FetchIncome(_ context.Context, userID string, _, _ int) (string, error) {
	if m.incomeErr != nil {
		return "", m.incomeErr
	}
	if m.incomeByUser != nil {
		return m.incomeByUser[userID], nil
	}
	return "0.00", nil
}

func (m *mockRepo) FetchExpensesByType(_ context.Context, userID string, _, _ int) ([]TypeAmount, error) {
	if m.expensesErr != nil {
		return nil, m.expensesErr
	}
	if m.expensesByUser != nil {
		return m.expensesByUser[userID], nil
	}
	return nil, nil
}

func (m *mockRepo) FetchSavings(_ context.Context, _ string, _, _ int) (SavingsRow, bool, error) {
	return m.savings, m.savingsFound, m.savingsErr
}

// --- buildExpenses ---

func TestBuildExpenses_IncomeZero(t *testing.T) {
	exp := buildExpenses(nil, "0.00")

	assert.Equal(t, "0.00", exp.Total)
	for _, typ := range allTypes {
		b := exp.ByType[typ]
		require.NotNil(t, b, "type %s missing", typ)
		assert.Equal(t, "0.00", b.Amount)
		assert.Equal(t, "0.00", b.Pct)
		assert.Equal(t, "under", b.Status, "type %s should be under when income is zero", typ)
	}
}

func TestBuildExpenses_StatusUnder(t *testing.T) {
	// necessity target = 50% of 1000 = 500; we spend 400 → under
	ta := []TypeAmount{{Type: "necessity", Amount: "400.00"}}
	exp := buildExpenses(ta, "1000.00")

	assert.Equal(t, "under", exp.ByType["necessity"].Status)
	assert.Equal(t, "40.00", exp.ByType["necessity"].Pct)
	assert.Equal(t, "50", exp.ByType["necessity"].TargetPct)
}

func TestBuildExpenses_StatusMet(t *testing.T) {
	// necessity target = 50% of 1000 = 500; we spend exactly 500 → met
	ta := []TypeAmount{{Type: "necessity", Amount: "500.00"}}
	exp := buildExpenses(ta, "1000.00")

	assert.Equal(t, "met", exp.ByType["necessity"].Status)
}

func TestBuildExpenses_StatusOver(t *testing.T) {
	// leisure target = 30% of 1000 = 300; we spend 400 → over
	ta := []TypeAmount{{Type: "leisure", Amount: "400.00"}}
	exp := buildExpenses(ta, "1000.00")

	assert.Equal(t, "over", exp.ByType["leisure"].Status)
}

func TestBuildExpenses_MissingTypeDefaultsToZero(t *testing.T) {
	// only necessity provided; investment, leisure, saving should be "0.00"
	ta := []TypeAmount{{Type: "necessity", Amount: "100.00"}}
	exp := buildExpenses(ta, "500.00")

	assert.Equal(t, "0.00", exp.ByType["investment"].Amount)
	assert.Equal(t, "under", exp.ByType["investment"].Status)
}

func TestBuildExpenses_TotalSumsAllTypes(t *testing.T) {
	ta := []TypeAmount{
		{Type: "necessity", Amount: "300.00"},
		{Type: "leisure", Amount: "150.00"},
	}
	exp := buildExpenses(ta, "1000.00")

	assert.Equal(t, "450.00", exp.Total)
}

// --- buildSavings ---

func TestBuildSavings_Under(t *testing.T) {
	s := buildSavings(&SavingsRow{GoalAmount: "1000.00", SavedAmount: "500.00"})
	assert.Equal(t, "under", s.Status)
	assert.Equal(t, "1000.00", s.Goal)
	assert.Equal(t, "500.00", s.Saved)
}

func TestBuildSavings_Met(t *testing.T) {
	s := buildSavings(&SavingsRow{GoalAmount: "500.00", SavedAmount: "500.00"})
	assert.Equal(t, "met", s.Status)
}

func TestBuildSavings_Over(t *testing.T) {
	s := buildSavings(&SavingsRow{GoalAmount: "500.00", SavedAmount: "600.00"})
	assert.Equal(t, "over", s.Status)
}

// --- mergeTypeAmounts ---

func TestMergeTypeAmounts_SumsCorrectly(t *testing.T) {
	a := []TypeAmount{{Type: "necessity", Amount: "200.00"}, {Type: "leisure", Amount: "100.00"}}
	b := []TypeAmount{{Type: "necessity", Amount: "300.00"}, {Type: "saving", Amount: "50.00"}}

	result := mergeTypeAmounts(a, b)

	m := map[string]string{}
	for _, ta := range result {
		m[ta.Type] = ta.Amount
	}

	assert.Equal(t, "500.00", m["necessity"])
	assert.Equal(t, "100.00", m["leisure"])
	assert.Equal(t, "50.00", m["saving"])
}

func TestMergeTypeAmounts_EmptySlices(t *testing.T) {
	result := mergeTypeAmounts(nil, nil)
	assert.Empty(t, result)
}

// --- addAmounts ---

func TestAddAmounts(t *testing.T) {
	assert.Equal(t, "300.00", addAmounts("100.00", "200.00"))
	assert.Equal(t, "0.00", addAmounts("0.00", "0.00"))
}

// --- targetPct ---

func TestTargetPct_KnownTypes(t *testing.T) {
	cases := map[string]float64{
		"necessity":  500, // 50% of 1000
		"leisure":    300, // 30% of 1000
		"investment": 200, // 20% of 1000
		"saving":     200, // 20% of 1000
	}
	for typ, want := range cases {
		assert.Equal(t, want, targetPct(typ, 1000), "type %s", typ)
	}
}

func TestTargetPct_UnknownType(t *testing.T) {
	assert.Equal(t, float64(0), targetPct("unknown", 1000))
}

// --- calcStatus ---

func TestCalcStatus(t *testing.T) {
	assert.Equal(t, "under", calcStatus(100, 200))
	assert.Equal(t, "met", calcStatus(200, 200))
	assert.Equal(t, "over", calcStatus(300, 200))
}

// --- Service.Get via mock ---

func TestService_Get_NoCouple(t *testing.T) {
	repo := &mockRepo{
		userInfo:     UserInfo{CoupleID: sql.NullString{Valid: false}},
		incomeByUser: map[string]string{"user1": "2000.00"},
		expensesByUser: map[string][]TypeAmount{
			"user1": {{Type: "necessity", Amount: "900.00"}},
		},
		savings:      SavingsRow{GoalAmount: "400.00", SavedAmount: "200.00"},
		savingsFound: true,
	}
	svc := &Service{repo: repo}

	resp, err := svc.Get(context.Background(), "user1", 2026, 10)

	require.NoError(t, err)
	assert.Equal(t, "2000.00", resp.Income)
	assert.Nil(t, resp.Couple)
	require.NotNil(t, resp.Savings)
	assert.Equal(t, "under", resp.Savings.Status)
}

func TestService_Get_WithCouple(t *testing.T) {
	repo := &mockRepo{
		userInfo:  UserInfo{CoupleID: sql.NullString{Valid: true, String: "couple1"}},
		partnerID: "user2",
		incomeByUser: map[string]string{
			"user1": "1000.00",
			"user2": "1000.00",
		},
		expensesByUser: map[string][]TypeAmount{
			"user1": {{Type: "necessity", Amount: "300.00"}},
			"user2": {{Type: "necessity", Amount: "200.00"}},
		},
	}
	svc := &Service{repo: repo}

	resp, err := svc.Get(context.Background(), "user1", 2026, 10)

	require.NoError(t, err)
	require.NotNil(t, resp.Couple)
	assert.Equal(t, "2000.00", resp.Couple.Income)
	// combined necessity = 500 out of 2000 income = 25% → under 50% target
	assert.Equal(t, "under", resp.Couple.Expenses.ByType["necessity"].Status)
}

func TestService_Get_NoSavingsGoal(t *testing.T) {
	repo := &mockRepo{
		userInfo:     UserInfo{CoupleID: sql.NullString{Valid: false}},
		incomeByUser: map[string]string{"user1": "1000.00"},
		savingsFound: false,
	}
	svc := &Service{repo: repo}

	resp, err := svc.Get(context.Background(), "user1", 2026, 10)

	require.NoError(t, err)
	assert.Nil(t, resp.Savings)
}

func TestService_Get_FetchUserInfoError(t *testing.T) {
	repo := &mockRepo{userInfoErr: errors.New("db down")}
	svc := &Service{repo: repo}

	_, err := svc.Get(context.Background(), "user1", 2026, 10)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db down")
}

func TestService_Get_FetchIncomeError(t *testing.T) {
	repo := &mockRepo{
		userInfo:  UserInfo{CoupleID: sql.NullString{Valid: false}},
		incomeErr: errors.New("income query failed"),
	}
	svc := &Service{repo: repo}

	_, err := svc.Get(context.Background(), "user1", 2026, 10)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "income query failed")
}

func TestService_Get_PartnerNoLongerInCouple(t *testing.T) {
	// partnerID == "" means partner left the couple; couple view should be nil
	repo := &mockRepo{
		userInfo:     UserInfo{CoupleID: sql.NullString{Valid: true, String: "couple1"}},
		partnerID:    "",
		incomeByUser: map[string]string{"user1": "1000.00"},
		savingsFound: false,
	}
	svc := &Service{repo: repo}

	resp, err := svc.Get(context.Background(), "user1", 2026, 10)

	require.NoError(t, err)
	assert.Nil(t, resp.Couple)
}
