package couple

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Invite(t *testing.T) {
	t.Run("user already in couple returns ErrAlreadyInCouple", func(t *testing.T) {
		repo := &mockRepository{
			isInCoupleFn: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Invite(context.Background(), "user-1")

		assert.ErrorIs(t, err, ErrAlreadyInCouple)
	})

	t.Run("success returns invite with 6-char alphanumeric code", func(t *testing.T) {
		invalidateCalled := false
		repo := &mockRepository{
			invalidatePreviousInvitesFn: func(_ context.Context, _ string) error {
				invalidateCalled = true
				return nil
			},
		}
		svc := &Service{repo: repo}

		resp, err := svc.Invite(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Len(t, resp.Code, inviteCodeLen)
		assert.True(t, invalidateCalled)
		assert.NotZero(t, resp.ExpiresAt)
	})

	t.Run("code collision: retries until unique code found", func(t *testing.T) {
		callCount := 0
		repo := &mockRepository{
			codeExistsFn: func(_ context.Context, _ string) (bool, error) {
				callCount++
				return callCount < 3, nil // first 2 collide, 3rd is unique
			},
		}
		svc := &Service{repo: repo}

		resp, err := svc.Invite(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Len(t, resp.Code, inviteCodeLen)
		assert.Equal(t, 3, callCount)
	})

	t.Run("invalidate error propagates", func(t *testing.T) {
		repo := &mockRepository{
			invalidatePreviousInvitesFn: func(_ context.Context, _ string) error {
				return errors.New("db error")
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Invite(context.Background(), "user-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})
}
