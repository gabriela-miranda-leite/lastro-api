package transaction

import "context"

type mockRepository struct {
	findByClientIDFn func(ctx context.Context, clientID string) (Transaction, error)
	categoryOwnerFn  func(ctx context.Context, categoryID string) (string, bool, error)
	insertFn         func(ctx context.Context, userID string, input *CreateInput) (Transaction, error)
	userHasCoupleFn  func(ctx context.Context, userID string) (bool, error)
}

func (m *mockRepository) findByClientID(ctx context.Context, clientID string) (Transaction, error) {
	if m.findByClientIDFn != nil {
		return m.findByClientIDFn(ctx, clientID)
	}
	return Transaction{}, ErrNotFound
}

func (m *mockRepository) categoryOwner(ctx context.Context, categoryID string) (ownerID string, owned bool, err error) {
	if m.categoryOwnerFn != nil {
		return m.categoryOwnerFn(ctx, categoryID)
	}
	return "", false, nil // system category by default
}

func (m *mockRepository) insert(ctx context.Context, userID string, input *CreateInput) (Transaction, error) {
	if m.insertFn != nil {
		return m.insertFn(ctx, userID, input)
	}
	return Transaction{
		ID:        "tx-1",
		ClientID:  input.ClientID,
		UserID:    userID,
		Amount:    "100.00",
		Direction: input.Direction,
		IsShared:  input.IsShared,
		Date:      input.Date,
	}, nil
}

func (m *mockRepository) userHasCouple(ctx context.Context, userID string) (bool, error) {
	if m.userHasCoupleFn != nil {
		return m.userHasCoupleFn(ctx, userID)
	}
	return false, nil
}
