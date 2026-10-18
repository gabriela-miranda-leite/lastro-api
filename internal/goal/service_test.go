package goal

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validCreate() CreateInput {
	return CreateInput{Year: 2026, Month: 10, GoalAmount: 1000}
}

func TestService_Create_Success(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	g, err := svc.Create(context.Background(), "user-1", validCreate())
	require.NoError(t, err)
	assert.Equal(t, "goal-1", g.ID)
}

func TestService_Create_InvalidYear(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	input := validCreate()
	input.Year = 0
	_, err := svc.Create(context.Background(), "user-1", input)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "year", valErr.Field)
}

func TestService_Create_InvalidMonth(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	input := validCreate()
	input.Month = 13
	_, err := svc.Create(context.Background(), "user-1", input)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "month", valErr.Field)
}

func TestService_Create_InvalidGoalAmount(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	input := validCreate()
	input.GoalAmount = 0
	_, err := svc.Create(context.Background(), "user-1", input)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "goal_amount", valErr.Field)
}

func TestService_Create_Conflict(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Goal, error) {
			return Goal{}, ErrConflict
		},
	}}
	_, err := svc.Create(context.Background(), "user-1", validCreate())
	assert.ErrorIs(t, err, ErrConflict)
}

func TestService_Create_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Goal, error) {
			return Goal{}, errors.New("db error")
		},
	}}
	_, err := svc.Create(context.Background(), "user-1", validCreate())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- List ---

func TestService_List_Under(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string, _ *ListInput) ([]Goal, error) {
			return []Goal{{GoalAmount: "1000.00", SavedAmount: "500.00"}}, nil
		},
	}}
	goals, err := svc.List(context.Background(), "user-1", ListInput{})
	require.NoError(t, err)
	assert.Equal(t, "under", goals[0].Status)
}

func TestService_List_Met(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string, _ *ListInput) ([]Goal, error) {
			return []Goal{{GoalAmount: "1000.00", SavedAmount: "1000.00"}}, nil
		},
	}}
	goals, err := svc.List(context.Background(), "user-1", ListInput{})
	require.NoError(t, err)
	assert.Equal(t, "met", goals[0].Status)
}

func TestService_List_Over(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string, _ *ListInput) ([]Goal, error) {
			return []Goal{{GoalAmount: "500.00", SavedAmount: "1000.00"}}, nil
		},
	}}
	goals, err := svc.List(context.Background(), "user-1", ListInput{})
	require.NoError(t, err)
	assert.Equal(t, "over", goals[0].Status)
}

func TestService_List_Empty(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	goals, err := svc.List(context.Background(), "user-1", ListInput{})
	require.NoError(t, err)
	assert.Empty(t, goals)
}

func TestService_List_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string, _ *ListInput) ([]Goal, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := svc.List(context.Background(), "user-1", ListInput{})
	require.Error(t, err)
}

// --- Update ---

func pf(v float64) *float64 { return &v }

func TestService_Update_Success(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	g, err := svc.Update(context.Background(), "user-1", "goal-1", UpdateInput{SavedAmount: pf(500)})
	require.NoError(t, err)
	assert.Equal(t, "goal-1", g.ID)
}

func TestService_Update_NegativeAmount(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "goal-1", UpdateInput{SavedAmount: pf(-1)})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "saved_amount", valErr.Field)
}

func TestService_Update_ZeroAllowed(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "goal-1", UpdateInput{SavedAmount: pf(0)})
	require.NoError(t, err)
}

func TestService_Update_NotFound(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Goal, error) {
			return Goal{}, ErrNotFound
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "goal-1", UpdateInput{SavedAmount: pf(100)})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_Update_Forbidden(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Goal, error) {
			return Goal{ID: "goal-1", UserID: "other-user"}, nil
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "goal-1", UpdateInput{SavedAmount: pf(100)})
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestService_Update_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		updateFn: func(_ context.Context, _ string, _ *UpdateInput) (Goal, error) {
			return Goal{}, errors.New("db error")
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "goal-1", UpdateInput{SavedAmount: pf(100)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}
