package couple

import "time"

// ErrAlreadyInCouple is returned when the user already belongs to a couple.
var ErrAlreadyInCouple = errorSentinel("already in couple")

// ErrInviteNotFound is returned when no active invite exists for the given code.
var ErrInviteNotFound = errorSentinel("invite not found")

// ErrInviteExpired is returned when the invite code has passed its expiry.
var ErrInviteExpired = errorSentinel("invite expired")

// ErrSelfJoin is returned when a user tries to accept their own invite.
var ErrSelfJoin = errorSentinel("cannot accept own invite")

type errorSentinel string

func (e errorSentinel) Error() string { return string(e) }

// InviteResponse is the payload returned after generating a new invite code.
type InviteResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

// JoinInput is the request body for POST /couples/join.
type JoinInput struct {
	Code string `json:"code" binding:"required"`
}

// JoinResponse is the payload returned after successfully joining a couple.
type JoinResponse struct {
	CoupleID string `json:"couple_id"`
}

// InviteRecord holds the invite data fetched from the database.
type InviteRecord struct {
	ID        string
	InviterID string
	ExpiresAt time.Time
	UsedAt    *time.Time
}
