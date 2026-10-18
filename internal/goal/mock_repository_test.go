package goal

import "context"

type mockRepository struct {
	insertFn   func(ctx context.Context, userID string, input *CreateInput) (Goal, error)
	listFn     func(ctx context.Context, userID string, input *ListInput) ([]Goal, error)
	findByIDFn func(ctx context.Context, id string) (Goal, error)
	updateFn   func(ctx context.Context, id string, input *UpdateInput) (Goal, error)
}

func (m *mockRepository) insert(ctx context.Context, userID string, input *CreateInput) (Goal, error) {
	if m.insertFn != nil {
		return m.insertFn(ctx, userID, input)
	}
	return Goal{ID: "goal-1", UserID: userID, Year: input.Year, Month: input.Month,
		GoalAmount: "1000.00", SavedAmount: "0.00"}, nil
}

func (m *mockRepository) list(ctx context.Context, userID string, input *ListInput) ([]Goal, error) {
	if m.listFn != nil {
		return m.listFn(ctx, userID, input)
	}
	return []Goal{}, nil
}

func (m *mockRepository) findByID(ctx context.Context, id string) (Goal, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return Goal{ID: id, UserID: "user-1", GoalAmount: "1000.00", SavedAmount: "0.00"}, nil
}

func (m *mockRepository) update(ctx context.Context, id string, input *UpdateInput) (Goal, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, id, input)
	}
	return Goal{ID: id, UserID: "user-1", GoalAmount: "1000.00", SavedAmount: "500.00"}, nil
}
