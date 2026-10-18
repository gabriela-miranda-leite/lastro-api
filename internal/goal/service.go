package goal

import (
	"context"
	"fmt"
)

// Service contains business logic for the goal domain.
type Service struct {
	repo repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a new monthly savings goal for the user.
func (s *Service) Create(ctx context.Context, userID string, input CreateInput) (Goal, error) {
	if err := validateCreate(&input); err != nil {
		return Goal{}, err
	}
	g, err := s.repo.insert(ctx, userID, &input)
	if err != nil {
		return Goal{}, fmt.Errorf("goal.service.Create: %w", err)
	}
	return g, nil
}

func validateCreate(input *CreateInput) error {
	if input.Year <= 0 {
		return &ValidationError{Field: "year", Message: "must be greater than zero"}
	}
	if input.Month < 1 || input.Month > 12 {
		return &ValidationError{Field: "month", Message: "must be between 1 and 12"}
	}
	if input.GoalAmount <= 0 {
		return &ValidationError{Field: "goal_amount", Message: "must be greater than zero"}
	}
	return nil
}
