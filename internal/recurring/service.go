package recurring

import (
	"context"
	"fmt"
)

// Service contains business logic for the recurring domain.
type Service struct {
	repo repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns all recurring expenses for the given user.
func (s *Service) List(ctx context.Context, userID string) ([]Recurring, error) {
	recs, err := s.repo.list(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("recurring.service.List: %w", err)
	}
	return recs, nil
}

// Create creates a new recurring expense.
func (s *Service) Create(ctx context.Context, userID string, input CreateInput) (Recurring, error) {
	if err := validateCreate(&input); err != nil {
		return Recurring{}, err
	}
	rec, err := s.repo.insert(ctx, userID, &input)
	if err != nil {
		return Recurring{}, fmt.Errorf("recurring.service.Create: %w", err)
	}
	return rec, nil
}

// Update edits a recurring expense. Only the owner may edit.
func (s *Service) Update(ctx context.Context, userID, id string, input UpdateInput) (Recurring, error) {
	if err := validateUpdate(&input); err != nil {
		return Recurring{}, err
	}
	existing, err := s.repo.findByID(ctx, id)
	if err != nil {
		return Recurring{}, fmt.Errorf("recurring.service.Update: %w", err)
	}
	if existing.UserID != userID {
		return Recurring{}, ErrForbidden
	}
	rec, err := s.repo.update(ctx, id, &input)
	if err != nil {
		return Recurring{}, fmt.Errorf("recurring.service.Update: %w", err)
	}
	return rec, nil
}

// Delete permanently removes a recurring expense. Only the owner may delete.
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	existing, err := s.repo.findByID(ctx, id)
	if err != nil {
		return fmt.Errorf("recurring.service.Delete: %w", err)
	}
	if existing.UserID != userID {
		return ErrForbidden
	}
	if err := s.repo.delete(ctx, id); err != nil {
		return fmt.Errorf("recurring.service.Delete: %w", err)
	}
	return nil
}

// UpsertOverride creates or updates a monthly override for a recurring expense.
// Only the owner may create/update an override.
func (s *Service) UpsertOverride(ctx context.Context, userID, recurringID string, input OverrideInput) (RecurringOverride, error) {
	if err := validateOverride(&input); err != nil {
		return RecurringOverride{}, err
	}
	existing, err := s.repo.findByID(ctx, recurringID)
	if err != nil {
		return RecurringOverride{}, fmt.Errorf("recurring.service.UpsertOverride: %w", err)
	}
	if existing.UserID != userID {
		return RecurringOverride{}, ErrForbidden
	}
	o, err := s.repo.upsertOverride(ctx, recurringID, &input)
	if err != nil {
		return RecurringOverride{}, fmt.Errorf("recurring.service.UpsertOverride: %w", err)
	}
	return o, nil
}

func validateCreate(input *CreateInput) error {
	if input.Amount <= 0 {
		return &ValidationError{Field: "amount", Message: "must be greater than zero"}
	}
	if input.DueDay < 1 || input.DueDay > 31 {
		return &ValidationError{Field: "due_day", Message: "must be between 1 and 31"}
	}
	return nil
}

func validateUpdate(input *UpdateInput) error {
	if input.Amount != nil && *input.Amount <= 0 {
		return &ValidationError{Field: "amount", Message: "must be greater than zero"}
	}
	if input.DueDay != nil && (*input.DueDay < 1 || *input.DueDay > 31) {
		return &ValidationError{Field: "due_day", Message: "must be between 1 and 31"}
	}
	return nil
}

func validateOverride(input *OverrideInput) error {
	if input.Year <= 0 {
		return &ValidationError{Field: "year", Message: "must be greater than zero"}
	}
	if input.Month < 1 || input.Month > 12 {
		return &ValidationError{Field: "month", Message: "must be between 1 and 12"}
	}
	if input.AmountOverride != nil && *input.AmountOverride <= 0 {
		return &ValidationError{Field: "amount_override", Message: "must be greater than zero"}
	}
	return nil
}
