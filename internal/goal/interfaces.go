package goal

import "context"

type repository interface {
	insert(ctx context.Context, userID string, input *CreateInput) (Goal, error)
	list(ctx context.Context, userID string, input *ListInput) ([]Goal, error)
	findByID(ctx context.Context, id string) (Goal, error)
	update(ctx context.Context, id string, input *UpdateInput) (Goal, error)
}
