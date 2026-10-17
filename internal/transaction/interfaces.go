package transaction

import "context"

type repository interface {
	findByClientID(ctx context.Context, clientID string) (Transaction, error)
	// categoryOwner returns (ownerID, owned, err). owned=false means system category (user_id IS NULL).
	categoryOwner(ctx context.Context, categoryID string) (ownerID string, owned bool, err error)
	insert(ctx context.Context, userID string, input *CreateInput) (Transaction, error)
	userHasCouple(ctx context.Context, userID string) (bool, error)
	// sameCouple returns true when both users belong to the same couple.
	sameCouple(ctx context.Context, userIDA, userIDB string) (bool, error)
	list(ctx context.Context, userID string, input *ListInput) ([]Transaction, error)
}
