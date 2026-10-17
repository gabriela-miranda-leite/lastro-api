package couple

import (
	"context"
	"time"
)

type mockRepository struct {
	isInCoupleFn                func(ctx context.Context, userID string) (bool, error)
	invalidatePreviousInvitesFn func(ctx context.Context, userID string) error
	createInviteFn              func(ctx context.Context, userID, code string) (InviteResponse, error)
	codeExistsFn                func(ctx context.Context, code string) (bool, error)
}

func (m *mockRepository) isInCouple(ctx context.Context, userID string) (bool, error) {
	if m.isInCoupleFn != nil {
		return m.isInCoupleFn(ctx, userID)
	}
	return false, nil
}

func (m *mockRepository) invalidatePreviousInvites(ctx context.Context, userID string) error {
	if m.invalidatePreviousInvitesFn != nil {
		return m.invalidatePreviousInvitesFn(ctx, userID)
	}
	return nil
}

func (m *mockRepository) createInvite(ctx context.Context, userID, code string) (InviteResponse, error) {
	if m.createInviteFn != nil {
		return m.createInviteFn(ctx, userID, code)
	}
	return InviteResponse{Code: code, ExpiresAt: time.Now().Add(24 * time.Hour)}, nil
}

func (m *mockRepository) codeExists(ctx context.Context, code string) (bool, error) {
	if m.codeExistsFn != nil {
		return m.codeExistsFn(ctx, code)
	}
	return false, nil
}
