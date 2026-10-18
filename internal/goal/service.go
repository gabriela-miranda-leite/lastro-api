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
	for i := range goals {
		result[i] = GoalWithStatus{Goal: goals[i], Status: calcStatus(goals[i].SavedAmount, goals[i].GoalAmount)}
	}
	return result, nil
}

// Update sets the saved_amount of a goal, verifying ownership.
func (s *Service) Update(ctx context.Context, userID, id string, input UpdateInput) (Goal, error) {
	if err := validateUpdate(&input); err != nil {
		return Goal{}, err
	}
	g, err := s.repo.findByID(ctx, id)
	if err != nil {
		return Goal{}, fmt.Errorf("goal.service.Update: %w", err)
	}
	if g.UserID != userID {
		return Goal{}, fmt.Errorf("goal.service.Update: %w", ErrForbidden)
	}
	updated, err := s.repo.update(ctx, id, &input)
	if err != nil {
		return Goal{}, fmt.Errorf("goal.service.Update: %w", err)
	}
	return updated, nil
}

// calcStatus compares saved vs goal amounts (NUMERIC strings from the DB).
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

func validateUpdate(input *UpdateInput) error {
	if *input.SavedAmount < 0 {
		return &ValidationError{Field: "saved_amount", Message: "must be zero or positive"}
	}
	return nil
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
