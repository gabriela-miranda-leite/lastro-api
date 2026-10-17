package couple

import "context"

type repository interface {
	isInCouple(ctx context.Context, userID string) (bool, error)
	invalidatePreviousInvites(ctx context.Context, userID string) error
	createInvite(ctx context.Context, userID, code string) (InviteResponse, error)
	codeExists(ctx context.Context, code string) (bool, error)
}
