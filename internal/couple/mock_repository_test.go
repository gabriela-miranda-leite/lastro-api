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
	findInviteByCodeFn          func(ctx context.Context, code string) (InviteRecord, error)
	joinCoupleFn                func(ctx context.Context, inviteID, inviterID, joinerID string) (JoinResponse, error)
	findCoupleIDFn              func(ctx context.Context, userID string) (string, error)
	findPartnerIDFn             func(ctx context.Context, userID, coupleID string) (string, error)
	unlinkFn                    func(ctx context.Context, userID, partnerID, coupleID string) error
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

func (m *mockRepository) findInviteByCode(ctx context.Context, code string) (InviteRecord, error) {
	if m.findInviteByCodeFn != nil {
		return m.findInviteByCodeFn(ctx, code)
	}
	return InviteRecord{}, ErrInviteNotFound
}

func (m *mockRepository) joinCouple(ctx context.Context, inviteID, inviterID, joinerID string) (JoinResponse, error) {
	if m.joinCoupleFn != nil {
		return m.joinCoupleFn(ctx, inviteID, inviterID, joinerID)
	}
	return JoinResponse{CoupleID: "couple-1"}, nil
}

func (m *mockRepository) findCoupleID(ctx context.Context, userID string) (string, error) {
	if m.findCoupleIDFn != nil {
		return m.findCoupleIDFn(ctx, userID)
	}
	return "", ErrNotInCouple
}

func (m *mockRepository) findPartnerID(ctx context.Context, userID, coupleID string) (string, error) {
	if m.findPartnerIDFn != nil {
		return m.findPartnerIDFn(ctx, userID, coupleID)
	}
	return "partner-1", nil
}

func (m *mockRepository) unlink(ctx context.Context, userID, partnerID, coupleID string) error {
	if m.unlinkFn != nil {
		return m.unlinkFn(ctx, userID, partnerID, coupleID)
	}
	return nil
}
