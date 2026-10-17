package transaction

import (
	"context"
	"fmt"
	"time"
)

// Service contains business logic for the transaction domain.
type Service struct {
	repo repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// Create creates a new transaction or returns the existing one if client_id already exists.
// The second return value is true when the transaction already existed (idempotent hit).
func (s *Service) Create(ctx context.Context, userID string, input *CreateInput) (Transaction, bool, error) {
	if err := s.validate(input); err != nil {
		return Transaction{}, false, err
	}

	if input.IsShared {
		hasCouple, err := s.repo.userHasCouple(ctx, userID)
		if err != nil {
			return Transaction{}, false, fmt.Errorf("transaction.service.Create: %w", err)
		}
		if !hasCouple {
			return Transaction{}, false, ErrSharedWithoutCouple
		}
	}

	if input.CategoryID != nil {
		ownerID, owned, err := s.repo.categoryOwner(ctx, *input.CategoryID)
		if err != nil {
			return Transaction{}, false, fmt.Errorf("transaction.service.Create: %w", err)
		}
		if owned && ownerID != userID {
			return Transaction{}, false, ErrCategoryForbidden
		}
	}

	existing, err := s.repo.findByClientID(ctx, input.ClientID)
	if err == nil {
		return existing, true, nil
	}
	if err != ErrNotFound {
		return Transaction{}, false, fmt.Errorf("transaction.service.Create: %w", err)
	}

	tx, err := s.repo.insert(ctx, userID, input)
	if err != nil {
		return Transaction{}, false, fmt.Errorf("transaction.service.Create: %w", err)
	}
	return tx, false, nil
}

func (s *Service) validate(input *CreateInput) error {
	if input.Amount <= 0 {
		return &ValidationError{Field: "amount", Message: "must be greater than zero"}
	}
	if input.Direction != "in" && input.Direction != "out" {
		return &ValidationError{Field: "direction", Message: `must be "in" or "out"`}
	}
	if _, err := time.Parse("2006-01-02", input.Date); err != nil {
		return &ValidationError{Field: "date", Message: "must be in YYYY-MM-DD format"}
	}
	return nil
}
