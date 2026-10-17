package recurring

import "context"

type mockRepository struct {
	listFn           func(ctx context.Context, userID string) ([]Recurring, error)
	findByIDFn       func(ctx context.Context, id string) (Recurring, error)
	insertFn         func(ctx context.Context, userID string, input *CreateInput) (Recurring, error)
	updateFn         func(ctx context.Context, id string, input *UpdateInput) (Recurring, error)
	deleteFn         func(ctx context.Context, id string) error
	upsertOverrideFn func(ctx context.Context, recurringID string, input *OverrideInput) (RecurringOverride, error)
}

func (m *mockRepository) list(ctx context.Context, userID string) ([]Recurring, error) {
	if m.listFn != nil {
		return m.listFn(ctx, userID)
	}
	return []Recurring{}, nil
}

func (m *mockRepository) findByID(ctx context.Context, id string) (Recurring, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return Recurring{}, ErrNotFound
}

func (m *mockRepository) insert(ctx context.Context, userID string, input *CreateInput) (Recurring, error) {
	if m.insertFn != nil {
		return m.insertFn(ctx, userID, input)
	}
	return Recurring{ID: "rec-1", UserID: userID, Name: input.Name}, nil
}

func (m *mockRepository) update(ctx context.Context, id string, input *UpdateInput) (Recurring, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, input)
	}
	return Recurring{ID: id}, nil
}

func (m *mockRepository) delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockRepository) upsertOverride(ctx context.Context, recurringID string, input *OverrideInput) (RecurringOverride, error) {
	if m.upsertOverrideFn != nil {
		return m.upsertOverrideFn(ctx, recurringID, input)
	}
	return RecurringOverride{ID: "ov-1", RecurringID: recurringID, Year: input.Year, Month: input.Month, Active: input.Active}, nil
}
