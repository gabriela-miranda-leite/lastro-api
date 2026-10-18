package category

import (
	"context"
	"fmt"
)

var validTypes = map[string]bool{
	"necessity":  true,
	"leisure":    true,
	"investment": true,
	"saving":     true,
}

// Service contains business logic for the category domain.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, userID string) ([]Category, error) {
	cats, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("category.service.List: %w", err)
	}
	return cats, nil
}

func (s *Service) Create(ctx context.Context, userID string, input CreateInput) (Category, error) {
	if !validTypes[input.Type] {
		return Category{}, &ValidationError{Field: "type", Message: "must be one of: necessity, leisure, investment, saving"}
	}
	c, err := s.repo.Insert(ctx, userID, input)
	if err != nil {
		return Category{}, fmt.Errorf("category.service.Create: %w", err)
	}
	return c, nil
}

func (s *Service) Update(ctx context.Context, userID, id string, input UpdateInput) (Category, error) {
	if input.Type != nil && !validTypes[*input.Type] {
		return Category{}, &ValidationError{Field: "type", Message: "must be one of: necessity, leisure, investment, saving"}
	}
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return Category{}, fmt.Errorf("category.service.Update: %w", err)
	}
	if existing.UserID != userID {
		return Category{}, ErrForbidden
	}
	c, err := s.repo.Update(ctx, id, input)
	if err != nil {
		return Category{}, fmt.Errorf("category.service.Update: %w", err)
	}
	return c, nil
}

func (s *Service) Delete(ctx context.Context, userID, id string) error {
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("category.service.Delete: %w", err)
	}
	if existing.UserID != userID {
		return ErrForbidden
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("category.service.Delete: %w", err)
	}
	return nil
}

// ValidationError carries a field-level validation failure.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }
