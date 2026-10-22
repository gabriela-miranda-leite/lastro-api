package purge_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/purge"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

func TestWorker_PurgeDeletesExpiredUsers(t *testing.T) {
	db := testhelpers.SetupTestDB(t)

	// User soft-deleted 31 days ago — past the 30-day retention window.
	var oldUserID string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (name, email, password_hash, deleted_at)
		 VALUES ('Old', 'old@purge.test', 'h', now() - interval '31 days')
		 RETURNING id`,
	).Scan(&oldUserID)
	require.NoError(t, err)

	// Give the user a transaction so purgeUser's inner deletes are exercised.
	_, err = db.ExecContext(context.Background(),
		`INSERT INTO transactions (id, user_id, amount, direction, date, client_id)
		 VALUES (gen_random_uuid(), $1, 1.00, 'EXPENSE', '2020-01-01', gen_random_uuid())`,
		oldUserID,
	)
	require.NoError(t, err)

	// User soft-deleted 5 days ago — within retention window, must NOT be purged.
	var recentUserID string
	err = db.QueryRowContext(context.Background(),
		`INSERT INTO users (name, email, password_hash, deleted_at)
		 VALUES ('Recent', 'recent@purge.test', 'h', now() - interval '5 days')
		 RETURNING id`,
	).Scan(&recentUserID)
	require.NoError(t, err)

	// Live user — must NOT be purged.
	var liveUserID string
	err = db.QueryRowContext(context.Background(),
		`INSERT INTO users (name, email, password_hash)
		 VALUES ('Live', 'live@purge.test', 'h')
		 RETURNING id`,
	).Scan(&liveUserID)
	require.NoError(t, err)

	w := purge.New(db)
	require.NoError(t, w.RunOnce(context.Background()))

	var count int

	// Expired user and their data must be gone.
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM users WHERE id = $1`, oldUserID).Scan(&count))
	assert.Equal(t, 0, count, "expired user must be hard-deleted")

	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id = $1`, oldUserID).Scan(&count))
	assert.Equal(t, 0, count, "expired user's transactions must be deleted")

	// Recent and live users must still exist.
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM users WHERE id = $1 OR id = $2`, recentUserID, liveUserID).Scan(&count))
	assert.Equal(t, 2, count, "non-expired users must not be purged")
}

func TestWorker_RunCancels(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	w := purge.New(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so the loop exits without waiting for the ticker

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}
}
