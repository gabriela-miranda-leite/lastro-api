package couple

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Unlink(t *testing.T) {
	t.Run("not in couple returns ErrNotInCouple", func(t *testing.T) {
		repo := &mockRepository{} // default findCoupleID returns ErrNotInCouple
		svc := &Service{repo: repo}

		err := svc.Unlink(context.Background(), "user-1")

		assert.ErrorIs(t, err, ErrNotInCouple)
	})

	t.Run("success calls unlink with correct args", func(t *testing.T) {
		unlinkCalled := false
		var gotUserID, gotPartnerID, gotCoupleID string
		repo := &mockRepository{
			findCoupleIDFn: func(_ context.Context, _ string) (string, error) {
				return "couple-abc", nil
			},
			findPartnerIDFn: func(_ context.Context, _, _ string) (string, error) {
				return "partner-1", nil
			},
			unlinkFn: func(_ context.Context, userID, partnerID, coupleID string) error {
				unlinkCalled = true
				gotUserID, gotPartnerID, gotCoupleID = userID, partnerID, coupleID
				return nil
			},
		}
		svc := &Service{repo: repo}

		err := svc.Unlink(context.Background(), "user-1")

		require.NoError(t, err)
		assert.True(t, unlinkCalled)
		assert.Equal(t, "user-1", gotUserID)
		assert.Equal(t, "partner-1", gotPartnerID)
		assert.Equal(t, "couple-abc", gotCoupleID)
	})

	t.Run("unlink with no partner (partner already deleted) still succeeds", func(t *testing.T) {
		repo := &mockRepository{
			findCoupleIDFn: func(_ context.Context, _ string) (string, error) {
				return "couple-abc", nil
			},
			findPartnerIDFn: func(_ context.Context, _, _ string) (string, error) {
				return "", nil // no partner found
			},
		}
		svc := &Service{repo: repo}

		err := svc.Unlink(context.Background(), "user-1")

		assert.NoError(t, err)
	})

	t.Run("findPartnerID error propagates", func(t *testing.T) {
		repo := &mockRepository{
			findCoupleIDFn: func(_ context.Context, _ string) (string, error) {
				return "couple-abc", nil
			},
			findPartnerIDFn: func(_ context.Context, _, _ string) (string, error) {
				return "", errors.New("db error")
			},
		}
		svc := &Service{repo: repo}

		err := svc.Unlink(context.Background(), "user-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("unlink repo error propagates", func(t *testing.T) {
		repo := &mockRepository{
			findCoupleIDFn: func(_ context.Context, _ string) (string, error) {
				return "couple-abc", nil
			},
			findPartnerIDFn: func(_ context.Context, _, _ string) (string, error) {
				return "partner-1", nil
			},
			unlinkFn: func(_ context.Context, _, _, _ string) error {
				return errors.New("tx error")
			},
		}
		svc := &Service{repo: repo}

		err := svc.Unlink(context.Background(), "user-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "tx error")
	})
}

func activeInvite(inviterID string) InviteRecord {
	return InviteRecord{
		ID:        "inv-1",
		InviterID: inviterID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func TestService_Join(t *testing.T) {
	t.Run("invite not found returns ErrInviteNotFound", func(t *testing.T) {
		repo := &mockRepository{} // default returns ErrInviteNotFound
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "XXXXXX")

		assert.ErrorIs(t, err, ErrInviteNotFound)
	})

	t.Run("findInviteByCode db error propagates", func(t *testing.T) {
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return InviteRecord{}, errors.New("db error")
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("expired invite returns ErrInviteExpired", func(t *testing.T) {
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return InviteRecord{
					ID:        "inv-1",
					InviterID: "user-1",
					ExpiresAt: time.Now().Add(-1 * time.Hour), // already past
				}, nil
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		assert.ErrorIs(t, err, ErrInviteExpired)
	})

	t.Run("used invite returns ErrInviteExpired", func(t *testing.T) {
		usedAt := time.Now().Add(-1 * time.Minute)
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return InviteRecord{
					ID:        "inv-1",
					InviterID: "user-1",
					ExpiresAt: time.Now().Add(24 * time.Hour),
					UsedAt:    &usedAt,
				}, nil
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		assert.ErrorIs(t, err, ErrInviteExpired)
	})

	t.Run("self-join returns ErrSelfJoin", func(t *testing.T) {
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return activeInvite("user-1"), nil
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-1", "ABCDEF")

		assert.ErrorIs(t, err, ErrSelfJoin)
	})

	t.Run("joiner already in couple returns ErrAlreadyInCouple", func(t *testing.T) {
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return activeInvite("user-1"), nil
			},
			isInCoupleFn: func(_ context.Context, _ string) (bool, error) {
				return true, nil
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		assert.ErrorIs(t, err, ErrAlreadyInCouple)
	})

	t.Run("isInCouple db error propagates", func(t *testing.T) {
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return activeInvite("user-1"), nil
			},
			isInCoupleFn: func(_ context.Context, _ string) (bool, error) {
				return false, errors.New("db error")
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("joinCouple repo error propagates", func(t *testing.T) {
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return activeInvite("user-1"), nil
			},
			joinCoupleFn: func(_ context.Context, _, _, _ string) (JoinResponse, error) {
				return JoinResponse{}, errors.New("tx error")
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "tx error")
	})

	t.Run("success calls joinCouple and returns couple_id", func(t *testing.T) {
		joinCalled := false
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return activeInvite("user-1"), nil
			},
			joinCoupleFn: func(_ context.Context, _, _, _ string) (JoinResponse, error) {
				joinCalled = true
				return JoinResponse{CoupleID: "couple-abc"}, nil
			},
		}
		svc := &Service{repo: repo}

		resp, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		require.NoError(t, err)
		assert.Equal(t, "couple-abc", resp.CoupleID)
		assert.True(t, joinCalled)
	})

	t.Run("joinCouple receives correct inviteID from found record", func(t *testing.T) {
		var gotInviteID string
		repo := &mockRepository{
			findInviteByCodeFn: func(_ context.Context, _ string) (InviteRecord, error) {
				return InviteRecord{ID: "inv-xyz", InviterID: "user-1", ExpiresAt: time.Now().Add(time.Hour)}, nil
			},
			joinCoupleFn: func(_ context.Context, inviteID, _, _ string) (JoinResponse, error) {
				gotInviteID = inviteID
				return JoinResponse{CoupleID: "couple-abc"}, nil
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Join(context.Background(), "user-2", "ABCDEF")

		require.NoError(t, err)
		assert.Equal(t, "inv-xyz", gotInviteID)
	})
}

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

	t.Run("isInCouple db error propagates", func(t *testing.T) {
		repo := &mockRepository{
			isInCoupleFn: func(_ context.Context, _ string) (bool, error) {
				return false, errors.New("db error")
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Invite(context.Background(), "user-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "db error")
	})

	t.Run("createInvite error propagates", func(t *testing.T) {
		repo := &mockRepository{
			isInCoupleFn: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			createInviteFn: func(_ context.Context, _, _ string) (InviteResponse, error) {
				return InviteResponse{}, errors.New("insert failed")
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Invite(context.Background(), "user-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insert failed")
	})

	t.Run("exhausting max retries on code collision returns error", func(t *testing.T) {
		repo := &mockRepository{
			isInCoupleFn: func(_ context.Context, _ string) (bool, error) {
				return false, nil
			},
			codeExistsFn: func(_ context.Context, _ string) (bool, error) {
				return true, nil // always collides
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Invite(context.Background(), "user-1")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate unique code")
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
