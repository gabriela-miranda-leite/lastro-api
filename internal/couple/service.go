package couple

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
)

const inviteCodeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
const inviteCodeLen = 6
const maxRetries = 10

// Service contains business logic for the couple domain.
type Service struct {
	repo repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Invite generates a unique 6-char invite code for the user, invalidating any previous active invite.
// Returns ErrAlreadyInCouple if the user already belongs to a couple.
func (s *Service) Invite(ctx context.Context, userID string) (InviteResponse, error) {
	inCouple, err := s.repo.isInCouple(ctx, userID)
	if err != nil {
		return InviteResponse{}, fmt.Errorf("couple.service.Invite: %w", err)
	}
	if inCouple {
		return InviteResponse{}, ErrAlreadyInCouple
	}

	if err := s.repo.invalidatePreviousInvites(ctx, userID); err != nil {
		return InviteResponse{}, fmt.Errorf("couple.service.Invite: %w", err)
	}

	code, err := s.generateUniqueCode(ctx)
	if err != nil {
		return InviteResponse{}, fmt.Errorf("couple.service.Invite: %w", err)
	}

	resp, err := s.repo.createInvite(ctx, userID, code)
	if err != nil {
		return InviteResponse{}, fmt.Errorf("couple.service.Invite: %w", err)
	}
	return resp, nil
}

func (s *Service) generateUniqueCode(ctx context.Context) (string, error) {
	alphabetLen := big.NewInt(int64(len(inviteCodeAlphabet)))
	for range maxRetries {
		b := make([]byte, inviteCodeLen)
		for i := range b {
			n, err := rand.Int(rand.Reader, alphabetLen)
			if err != nil {
				return "", fmt.Errorf("couple.service.generateUniqueCode: %w", err)
			}
			b[i] = inviteCodeAlphabet[n.Int64()]
		}
		code := string(b)

		exists, err := s.repo.codeExists(ctx, code)
		if err != nil {
			return "", err
		}
		if !exists {
			return code, nil
		}
	}
	return "", fmt.Errorf("couple.service.generateUniqueCode: failed to generate unique code after %d attempts", maxRetries)
}
