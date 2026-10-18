package goal

import (
	"context"
	"fmt"
	"strconv"
)

// Service contains business logic for the goal domain.
type Service struct {
	repo repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns the user's savings goals, optionally filtered by year and/or month.
func (s *Service) List(ctx context.Context, userID string, input ListInput) ([]GoalWithStatus, error) {
	goals, err := s.repo.list(ctx, userID, &input)
	if err != nil {
		return nil, fmt.Errorf("goal.service.List: %w", err)
	}
	result := make([]GoalWithStatus, len(goals))
	for i, g := range goals {
		result[i] = GoalWithStatus{Goal: g, Status: calcStatus(g.SavedAmount, g.GoalAmount)}
	}
	return result, nil
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

// calcStatus compares saved vs goal amounts (NUMERIC strings from the DB) and returns the status.
func calcStatus(saved, goal string) string {
	s, _ := strconv.ParseFloat(saved, 64)
	g, _ := strconv.ParseFloat(goal, 64)
	switch {
	case s < g:
		return "under"
	case s == g:
		return "met"
	default:
		return "over"
	}
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
