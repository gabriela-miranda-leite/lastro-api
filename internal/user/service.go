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

func (s *Service) Update(ctx context.Context, userID string, input UpdateInput) (MeResponse, error) {
	if input.Name != nil && *input.Name == "" {
		return MeResponse{}, fmt.Errorf("user.service.Update: name: %w", ErrInvalidInput)
	}
	if input.MonthlyIncome != nil && *input.MonthlyIncome < 0 {
		return MeResponse{}, fmt.Errorf("user.service.Update: monthly_income: %w", ErrInvalidInput)
	}
	if input.SavingGoalPct != nil && (*input.SavingGoalPct < 0 || *input.SavingGoalPct > 100) {
		return MeResponse{}, fmt.Errorf("user.service.Update: saving_goal_pct: %w", ErrInvalidInput)
	}

	if err := s.repo.update(ctx, userID, input); err != nil {
		return MeResponse{}, fmt.Errorf("user.service.Update: %w", err)
	}

	return s.Me(ctx, userID)
}
