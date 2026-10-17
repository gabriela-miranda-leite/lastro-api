package recurring

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fptr(f float64) *float64 { return &f }
func iptr(i int) *int         { return &i }
func bptr(b bool) *bool       { return &b }

func validCreate() CreateInput {
	return CreateInput{Name: "Aluguel", Amount: 1500, DueDay: 10}
}

// --- Create ---

func TestService_Create_AmountZero(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	input := validCreate()
	input.Amount = 0
	_, err := svc.Create(context.Background(), "user-1", input)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "amount", valErr.Field)
}

func TestService_Create_AmountNegative(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	input := validCreate()
	input.Amount = -10
	_, err := svc.Create(context.Background(), "user-1", input)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "amount", valErr.Field)
}

func TestService_Create_DueDayZero(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	input := validCreate()
	input.DueDay = 0
	_, err := svc.Create(context.Background(), "user-1", input)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "due_day", valErr.Field)
}

func TestService_Create_DueDayOver31(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	input := validCreate()
	input.DueDay = 32
	_, err := svc.Create(context.Background(), "user-1", input)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "due_day", valErr.Field)
}

func TestService_Create_Success(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	rec, err := svc.Create(context.Background(), "user-1", validCreate())
	require.NoError(t, err)
	assert.Equal(t, "rec-1", rec.ID)
}

func TestService_Create_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Recurring, error) {
			return Recurring{}, errors.New("db error")
		},
	}}
	_, err := svc.Create(context.Background(), "user-1", validCreate())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- Update ---

func TestService_Update_NotFound(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "rec-999", UpdateInput{})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_Update_Forbidden(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "other"}, nil
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "rec-1", UpdateInput{})
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestService_Update_InvalidAmount(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "rec-1", UpdateInput{Amount: fptr(-1)})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "amount", valErr.Field)
}

func TestService_Update_InvalidDueDay(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "rec-1", UpdateInput{DueDay: iptr(0)})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "due_day", valErr.Field)
}

func TestService_Update_Success(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "user-1"}, nil
		},
		updateFn: func(_ context.Context, id string, _ *UpdateInput) (Recurring, error) {
			return Recurring{ID: id, UserID: "user-1", Active: false}, nil
		},
	}}
	rec, err := svc.Update(context.Background(), "user-1", "rec-1", UpdateInput{Active: bptr(false)})
	require.NoError(t, err)
	assert.False(t, rec.Active)
}

func TestService_Update_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "user-1"}, nil
		},
		updateFn: func(_ context.Context, _ string, _ *UpdateInput) (Recurring, error) {
			return Recurring{}, errors.New("db error")
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "rec-1", UpdateInput{Amount: fptr(100)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- Delete ---

func TestService_Delete_NotFound(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	err := svc.Delete(context.Background(), "user-1", "rec-999")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_Delete_Forbidden(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "other"}, nil
		},
	}}
	err := svc.Delete(context.Background(), "user-1", "rec-1")
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestService_Delete_Success(t *testing.T) {
	deleted := false
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "user-1"}, nil
		},
		deleteFn: func(_ context.Context, _ string) error {
			deleted = true
			return nil
		},
	}}
	err := svc.Delete(context.Background(), "user-1", "rec-1")
	require.NoError(t, err)
	assert.True(t, deleted)
}

func TestService_Delete_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "user-1"}, nil
		},
		deleteFn: func(_ context.Context, _ string) error {
			return errors.New("db error")
		},
	}}
	err := svc.Delete(context.Background(), "user-1", "rec-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- UpsertOverride ---

func TestService_UpsertOverride_NotFound(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.UpsertOverride(context.Background(), "user-1", "rec-999", OverrideInput{Year: 2026, Month: 10})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_UpsertOverride_Forbidden(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "other"}, nil
		},
	}}
	_, err := svc.UpsertOverride(context.Background(), "user-1", "rec-1", OverrideInput{Year: 2026, Month: 10})
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestService_UpsertOverride_InvalidMonth(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.UpsertOverride(context.Background(), "user-1", "rec-1", OverrideInput{Year: 2026, Month: 13})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "month", valErr.Field)
}

func TestService_UpsertOverride_InvalidYear(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.UpsertOverride(context.Background(), "user-1", "rec-1", OverrideInput{Year: 0, Month: 5})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "year", valErr.Field)
}

func TestService_UpsertOverride_InvalidAmountOverride(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	amt := -10.0
	_, err := svc.UpsertOverride(context.Background(), "user-1", "rec-1", OverrideInput{Year: 2026, Month: 5, AmountOverride: &amt})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "amount_override", valErr.Field)
}

func TestService_UpsertOverride_Success(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "user-1"}, nil
		},
	}}
	o, err := svc.UpsertOverride(context.Background(), "user-1", "rec-1", OverrideInput{Year: 2026, Month: 10, Active: false})
	require.NoError(t, err)
	assert.Equal(t, "rec-1", o.RecurringID)
	assert.Equal(t, 2026, o.Year)
	assert.Equal(t, 10, o.Month)
}

func TestService_UpsertOverride_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Recurring, error) {
			return Recurring{ID: "rec-1", UserID: "user-1"}, nil
		},
		upsertOverrideFn: func(_ context.Context, _ string, _ *OverrideInput) (RecurringOverride, error) {
			return RecurringOverride{}, errors.New("db error")
		},
	}}
	_, err := svc.UpsertOverride(context.Background(), "user-1", "rec-1", OverrideInput{Year: 2026, Month: 10})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- List ---

func TestService_List_Success(t *testing.T) {
	expected := []Recurring{{ID: "rec-1"}, {ID: "rec-2"}}
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string) ([]Recurring, error) {
			return expected, nil
		},
	}}
	recs, err := svc.List(context.Background(), "user-1")
	require.NoError(t, err)
	assert.Equal(t, expected, recs)
}

func TestService_List_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string) ([]Recurring, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := svc.List(context.Background(), "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}
