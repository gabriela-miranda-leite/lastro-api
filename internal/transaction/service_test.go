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

// --- List ---

func listInput() ListInput {
	return ListInput{Year: 2026, Month: 10}
}

func TestService_List_MissingYear(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.List(context.Background(), "user-1", ListInput{Month: 10})
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "year/month", valErr.Field)
}

func TestService_List_MissingMonth(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.List(context.Background(), "user-1", ListInput{Year: 2026})
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "year/month", valErr.Field)
}

func TestService_List_InvalidMonth(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.List(context.Background(), "user-1", ListInput{Year: 2026, Month: 13})
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "month", valErr.Field)
}

func TestService_List_InvalidDirection(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	dir := "sideways"
	_, err := svc.List(context.Background(), "user-1", ListInput{Year: 2026, Month: 10, Direction: &dir})
	require.Error(t, err)
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "direction", valErr.Field)
}

func TestService_List_OtherUserNotInCouple(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		sameCoupleFn: func(_ context.Context, _, _ string) (bool, error) { return false, nil },
	}}
	other := "user-2"
	input := listInput()
	input.UserID = &other
	_, err := svc.List(context.Background(), "user-1", input)
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestService_List_OtherUserInCouple(t *testing.T) {
	tx := Transaction{ID: "tx-partner"}
	svc := &Service{repo: &mockRepository{
		sameCoupleFn: func(_ context.Context, _, _ string) (bool, error) { return true, nil },
		listFn: func(_ context.Context, userID string, _ *ListInput) ([]Transaction, error) {
			assert.Equal(t, "user-2", userID)
			return []Transaction{tx}, nil
		},
	}}
	other := "user-2"
	input := listInput()
	input.UserID = &other
	txs, err := svc.List(context.Background(), "user-1", input)
	require.NoError(t, err)
	require.Len(t, txs, 1)
	assert.Equal(t, "tx-partner", txs[0].ID)
}

func TestService_List_SameUserNoCoupleLookup(t *testing.T) {
	coupleCalled := false
	svc := &Service{repo: &mockRepository{
		sameCoupleFn: func(_ context.Context, _, _ string) (bool, error) {
			coupleCalled = true
			return false, nil
		},
	}}
	self := "user-1"
	input := listInput()
	input.UserID = &self
	_, err := svc.List(context.Background(), "user-1", input)
	require.NoError(t, err)
	assert.False(t, coupleCalled, "sameCouple must not be called when user_id == caller")
}

func TestService_List_Success(t *testing.T) {
	expected := []Transaction{{ID: "tx-1"}, {ID: "tx-2"}}
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string, _ *ListInput) ([]Transaction, error) {
			return expected, nil
		},
	}}
	txs, err := svc.List(context.Background(), "user-1", listInput())
	require.NoError(t, err)
	assert.Equal(t, expected, txs)
}

func TestService_List_RepoError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		listFn: func(_ context.Context, _ string, _ *ListInput) ([]Transaction, error) {
			return nil, errors.New("db error")
		},
	}}
	_, err := svc.List(context.Background(), "user-1", listInput())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestService_List_SameCoupleLookupError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		sameCoupleFn: func(_ context.Context, _, _ string) (bool, error) {
			return false, errors.New("db error")
		},
	}}
	other := "user-2"
	input := listInput()
	input.UserID = &other
	_, err := svc.List(context.Background(), "user-1", input)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

// --- Update ---

func fptr(f float64) *float64 { return &f }
func bptr(b bool) *bool       { return &b }

func TestService_Update_NotFound(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "tx-999", UpdateInput{})
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_Update_Forbidden(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "other-user"}, nil
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{})
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestService_Update_AmountZero(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{Amount: fptr(0)})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "amount", valErr.Field)
}

func TestService_Update_AmountNegative(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	_, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{Amount: fptr(-5)})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "amount", valErr.Field)
}

func TestService_Update_InvalidDate(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	d := "31/10/2026"
	_, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{Date: &d})
	var valErr *ValidationError
	require.ErrorAs(t, err, &valErr)
	assert.Equal(t, "date", valErr.Field)
}

func TestService_Update_CategoryForbidden(t *testing.T) {
	cat := "cat-1"
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "user-1"}, nil
		},
		categoryOwnerFn: func(_ context.Context, _ string) (string, bool, error) {
			return "other-user", true, nil
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{CategoryID: &cat})
	assert.ErrorIs(t, err, ErrCategoryForbidden)
}

func TestService_Update_Success(t *testing.T) {
	updated := Transaction{ID: "tx-1", UserID: "user-1", Amount: "200.00"}
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "user-1"}, nil
		},
		updateFn: func(_ context.Context, _, _ string, _ *UpdateInput) (Transaction, error) {
			return updated, nil
		},
	}}
	tx, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{Amount: fptr(200)})
	require.NoError(t, err)
	assert.Equal(t, "200.00", tx.Amount)
}

func TestService_Update_RepoUpdateError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "user-1"}, nil
		},
		updateFn: func(_ context.Context, _, _ string, _ *UpdateInput) (Transaction, error) {
			return Transaction{}, errors.New("db error")
		},
	}}
	_, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{Amount: fptr(50)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestService_Update_IsSharedToggle(t *testing.T) {
	var capturedInput *UpdateInput
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "user-1", IsShared: false}, nil
		},
		updateFn: func(_ context.Context, _, _ string, input *UpdateInput) (Transaction, error) {
			capturedInput = input
			return Transaction{ID: "tx-1", IsShared: true}, nil
		},
	}}
	tx, err := svc.Update(context.Background(), "user-1", "tx-1", UpdateInput{IsShared: bptr(true)})
	require.NoError(t, err)
	assert.True(t, tx.IsShared)
	require.NotNil(t, capturedInput.IsShared)
	assert.True(t, *capturedInput.IsShared)
}

// --- Delete ---

func TestService_Delete_NotFound(t *testing.T) {
	svc := &Service{repo: &mockRepository{}}
	err := svc.Delete(context.Background(), "user-1", "tx-999")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestService_Delete_Forbidden(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "other-user"}, nil
		},
	}}
	err := svc.Delete(context.Background(), "user-1", "tx-1")
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestService_Delete_SoftDeleteError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "user-1"}, nil
		},
		softDeleteFn: func(_ context.Context, _ *Transaction, _ string) error {
			return errors.New("db error")
		},
	}}
	err := svc.Delete(context.Background(), "user-1", "tx-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestService_Delete_Success(t *testing.T) {
	var capturedTx *Transaction
	var capturedCaller string
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{ID: "tx-1", UserID: "user-1", Amount: "50.00", Date: "2026-10-01"}, nil
		},
		softDeleteFn: func(_ context.Context, tx *Transaction, callerID string) error {
			capturedTx = tx
			capturedCaller = callerID
			return nil
		},
	}}
	err := svc.Delete(context.Background(), "user-1", "tx-1")
	require.NoError(t, err)
	require.NotNil(t, capturedTx)
	assert.Equal(t, "tx-1", capturedTx.ID)
	assert.Equal(t, "50.00", capturedTx.Amount)
	assert.Equal(t, "user-1", capturedCaller)
}

func TestService_Delete_FindByIDError(t *testing.T) {
	svc := &Service{repo: &mockRepository{
		findByIDFn: func(_ context.Context, _ string) (Transaction, error) {
			return Transaction{}, errors.New("db error")
		},
	}}
	err := svc.Delete(context.Background(), "user-1", "tx-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")
}

func TestValidationError_Error(t *testing.T) {
	e := &ValidationError{Field: "amount", Message: "must be positive"}
	assert.Equal(t, "amount: must be positive", e.Error())
}
