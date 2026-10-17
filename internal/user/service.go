package user

import (
	"context"
	"fmt"
)

// Service contains business logic for the user domain.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Me(ctx context.Context, userID string) (MeResponse, error) {
	me, err := s.repo.findByID(ctx, userID)
	if err != nil {
		return MeResponse{}, fmt.Errorf("user.service.Me: %w", err)
	}
	return me, nil
}
