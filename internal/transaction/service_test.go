package transaction

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr(s string) *string { return &s }

func validInput() CreateInput {
	return CreateInput{
		ClientID:  "550e8400-e29b-41d4-a716-446655440000",
		Amount:    100.0,
		Direction: "out",
		IsShared:  false,
		Date:      "2026-10-17",
	}
}

func validInputPtr() *CreateInput { i := validInput(); return &i }

// --- validation ---

func TestService_Create_Validation(t *testing.T) {
	tests := []struct {
		name      string
		input     CreateInput
		wantField string
		wantMsg   string
	}{
		{
			name:      "amount zero",
			input:     func() CreateInput { i := validInput(); i.Amount = 0; return i }(),
			wantField: "amount",
			wantMsg:   "greater than zero",
		},
		{
			name:      "amount negative",
			input:     func() CreateInput { i := validInput(); i.Amount = -1; return i }(),
			wantField: "amount",
			wantMsg:   "greater than zero",
		},
		{
			name:      "direction invalid",
			input:     func() CreateInput { i := validInput(); i.Direction = "sideways"; return i }(),
			wantField: "direction",
			wantMsg:   `"in" or "out"`,
		},
		{
			name:      "date wrong format",
			input:     func() CreateInput { i := validInput(); i.Date = "17/10/2026"; return i }(),
			wantField: "date",
			wantMsg:   "YYYY-MM-DD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{repo: &mockRepository{}}

			_, _, err := svc.Create(context.Background(), "user-1", &tt.input)

			require.Error(t, err)
			var valErr *ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Field)
			assert.Contains(t, valErr.Message, tt.wantMsg)
		})
	}
}

// --- is_shared ---

func TestService_Create_SharedWithoutCouple(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		userHasCoupleFn: func(_ context.Context, _ string) (bool, error) { return false, nil },
	}}
	input := validInput()
	input.IsShared = true

	_, _, err := svc.Create(context.Background(), "user-1", &input)

	assert.ErrorIs(t, err, ErrSharedWithoutCouple)
}

func TestService_Create_SharedWithCouple(t *testing.T) {
	insertCalled := false
	svc := &Service{repo: &mockRepository{
		userHasCoupleFn: func(_ context.Context, _ string) (bool, error) { return true, nil },
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Transaction, error) {
			insertCalled = true
			return Transaction{ID: "tx-1"}, nil
		},
	}}
	input := validInput()
	input.IsShared = true

	_, existed, err := svc.Create(context.Background(), "user-1", &input)

	require.NoError(t, err)
	assert.False(t, existed)
	assert.True(t, insertCalled)
}

func TestService_Create_SharedCoupleCheckError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		userHasCoupleFn: func(_ context.Context, _ string) (bool, error) {
			return false, errors.New("db error")
		},
	}}
	input := validInput()
	input.IsShared = true

	_, _, err := svc.Create(context.Background(), "user-1", &input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- category_id ---

func TestService_Create_CategoryForbidden(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		categoryOwnerFn: func(_ context.Context, _ string) (string, bool, error) {
			return "other-user", true, nil
		},
	}}
	input := validInput()
	input.CategoryID = ptr("cat-1")

	_, _, err := svc.Create(context.Background(), "user-1", &input)

	assert.ErrorIs(t, err, ErrCategoryForbidden)
}

func TestService_Create_CategoryOwnedBySameUser(t *testing.T) {
	insertCalled := false
	svc := &Service{repo: &mockRepository{
		categoryOwnerFn: func(_ context.Context, _ string) (string, bool, error) {
			return "user-1", true, nil
		},
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Transaction, error) {
			insertCalled = true
			return Transaction{ID: "tx-1"}, nil
		},
	}}
	input := validInput()
	input.CategoryID = ptr("cat-1")

	_, _, err := svc.Create(context.Background(), "user-1", &input)

	require.NoError(t, err)
	assert.True(t, insertCalled)
}

func TestService_Create_SystemCategoryAllowed(t *testing.T) {
	insertCalled := false
	svc := &Service{repo: &mockRepository{
		categoryOwnerFn: func(_ context.Context, _ string) (string, bool, error) {
			return "", false, nil // system category
		},
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Transaction, error) {
			insertCalled = true
			return Transaction{ID: "tx-1"}, nil
		},
	}}
	input := validInput()
	input.CategoryID = ptr("cat-sys")

	_, _, err := svc.Create(context.Background(), "user-1", &input)

	require.NoError(t, err)
	assert.True(t, insertCalled)
}

func TestService_Create_CategoryCheckError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		categoryOwnerFn: func(_ context.Context, _ string) (string, bool, error) {
			return "", false, errors.New("db error")
		},
	}}
	input := validInput()
	input.CategoryID = ptr("cat-1")

	_, _, err := svc.Create(context.Background(), "user-1", &input)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- idempotency ---

func TestService_Create_IdempotentReturnsExisting(t *testing.T) {
	existing := Transaction{ID: "tx-existing", ClientID: "550e8400-e29b-41d4-a716-446655440000"}
	insertCalled := false
	svc := &Service{repo: &mockRepository{
		findByClientIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return existing, nil
		},
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Transaction, error) {
			insertCalled = true
			return Transaction{}, nil
		},
	}}

	tx, existed, err := svc.Create(context.Background(), "user-1", validInputPtr())

	require.NoError(t, err)
	assert.True(t, existed)
	assert.Equal(t, "tx-existing", tx.ID)
	assert.False(t, insertCalled, "insert must not be called on idempotent hit")
}

func TestService_Create_FindByClientIDDbError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByClientIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{}, errors.New("db error")
		},
	}}

	_, _, err := svc.Create(context.Background(), "user-1", validInputPtr())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- success ---

func TestService_Create_Success(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}

	tx, existed, err := svc.Create(context.Background(), "user-1", validInputPtr())

	require.NoError(t, err)
	assert.False(t, existed)
	assert.Equal(t, "tx-1", tx.ID)
}

func TestService_Create_InsertError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		insertFn: func(_ context.Context, _ string, _ *CreateInput) (Transaction, error) {
			return Transaction{}, errors.New("insert error")
		},
	}}

	_, _, err := svc.Create(context.Background(), "user-1", validInputPtr())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "insert error")
}
