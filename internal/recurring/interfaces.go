package recurring

import "context"

type repository interface {
	list(ctx context.Context, userID string) ([]Recurring, error)
	findByID(ctx context.Context, id string) (Recurring, error)
	insert(ctx context.Context, userID string, input *CreateInput) (Recurring, error)
	update(ctx context.Context, id string, input *UpdateInput) (Recurring, error)
	delete(ctx context.Context, id string) error
	upsertOverride(ctx context.Context, recurringID string, input *OverrideInput) (RecurringOverride, error)
}
