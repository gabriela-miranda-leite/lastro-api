package transaction

import "context"

type mockRepository struct {
	findByClientIDFn func(ctx context.Context, clientID string) (Transaction, error)
	categoryOwnerFn  func(ctx context.Context, categoryID string) (string, bool, error)
	insertFn         func(ctx context.Context, userID string, input *CreateInput) (Transaction, error)
	userHasCoupleFn  func(ctx context.Context, userID string) (bool, error)
	sameCoupleFn     func(ctx context.Context, userIDA, userIDB string) (bool, error)
	listFn           func(ctx context.Context, userID string, input *ListInput) ([]Transaction, error)
	findByIDFn       func(ctx context.Context, id string) (Transaction, error)
	updateFn         func(ctx context.Context, id, userID string, input *UpdateInput) (Transaction, error)
	softDeleteFn     func(ctx context.Context, tx *Transaction, deletedByUserID string) error
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

func (m *mockRepository) sameCouple(ctx context.Context, userIDA, userIDB string) (bool, error) {
	if m.sameCoupleFn != nil {
		return m.sameCoupleFn(ctx, userIDA, userIDB)
	}
	return false, nil
}

func (m *mockRepository) list(ctx context.Context, userID string, input *ListInput) ([]Transaction, error) {
	if m.listFn != nil {
		return m.listFn(ctx, userID, input)
	}
	return []Transaction{}, nil
}

func (m *mockRepository) findByID(ctx context.Context, id string) (Transaction, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return Transaction{}, ErrNotFound
}

func (m *mockRepository) update(ctx context.Context, id, userID string, input *UpdateInput) (Transaction, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, userID, input)
	}
	return Transaction{ID: id, UserID: userID}, nil
}

func (m *mockRepository) softDelete(ctx context.Context, tx *Transaction, deletedByUserID string) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, tx, deletedByUserID)
	}
	return nil
}
