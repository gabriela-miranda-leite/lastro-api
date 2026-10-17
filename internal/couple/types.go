package couple

import "time"

// ErrAlreadyInCouple is returned when the user already belongs to a couple.
var ErrAlreadyInCouple = errorSentinel("already in couple")

// ErrInviteNotFound is returned when no active invite exists for the given code.
var ErrInviteNotFound = errorSentinel("invite not found")

type errorSentinel string

func (e errorSentinel) Error() string { return string(e) }

// InviteResponse is the payload returned after generating a new invite code.
type InviteResponse struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}
