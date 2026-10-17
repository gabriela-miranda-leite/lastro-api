package couple_test

import (
	"context"
	"errors"

	"github.com/gabriela-miranda-leite/lastro-api/internal/couple"
)

// errInternal is a generic unexpected error for handler error-path tests.
var errInternal = errors.New("unexpected db failure")

type mockService struct {
	inviteFn func(ctx context.Context, userID string) (couple.InviteResponse, error)
	joinFn   func(ctx context.Context, joinerID, code string) (couple.JoinResponse, error)
	unlinkFn func(ctx context.Context, userID string) error
}

func (m *mockService) Invite(ctx context.Context, userID string) (couple.InviteResponse, error) {
	if m.inviteFn != nil {
		return m.inviteFn(ctx, userID)
	}
	return couple.InviteResponse{}, nil
}

func (m *mockService) Join(ctx context.Context, joinerID, code string) (couple.JoinResponse, error) {
	if m.joinFn != nil {
		return m.joinFn(ctx, joinerID, code)
	}
	return couple.JoinResponse{}, nil
}

func (m *mockService) Unlink(ctx context.Context, userID string) error {
	if m.unlinkFn != nil {
		return m.unlinkFn(ctx, userID)
	}
	return nil
}
