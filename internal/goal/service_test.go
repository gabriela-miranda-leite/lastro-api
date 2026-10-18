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
