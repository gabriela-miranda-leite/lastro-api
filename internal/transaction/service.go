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

// Delete soft-deletes a transaction and writes an audit log entry.
func (s *Service) Delete(ctx context.Context, userID, id string) error {
	tx, err := s.repo.findByID(ctx, id)
	if err != nil {
		return fmt.Errorf("transaction.service.Delete: %w", err)
	}
	if tx.UserID != userID {
		return ErrForbidden
	}
	if err := s.repo.softDelete(ctx, &tx, userID); err != nil {
		return fmt.Errorf("transaction.service.Delete: %w", err)
	}
	return nil
}

// Update edits an existing transaction. Only the owner may edit.
func (s *Service) Update(ctx context.Context, userID, id string, input UpdateInput) (Transaction, error) {
	if input.Amount != nil && *input.Amount <= 0 {
		return Transaction{}, &ValidationError{Field: "amount", Message: "must be greater than zero"}
	}
	if input.Date != nil {
		if _, err := time.Parse("2006-01-02", *input.Date); err != nil {
			return Transaction{}, &ValidationError{Field: "date", Message: "must be in YYYY-MM-DD format"}
		}
	}

	tx, err := s.repo.findByID(ctx, id)
	if err != nil {
		return Transaction{}, fmt.Errorf("transaction.service.Update: %w", err)
	}
	if tx.UserID != userID {
		return Transaction{}, ErrForbidden
	}

	if input.CategoryID != nil {
		ownerID, owned, err := s.repo.categoryOwner(ctx, *input.CategoryID)
		if err != nil {
			return Transaction{}, fmt.Errorf("transaction.service.Update: %w", err)
		}
		if owned && ownerID != userID {
			return Transaction{}, ErrCategoryForbidden
		}
	}

	updated, err := s.repo.update(ctx, id, userID, &input)
	if err != nil {
		return Transaction{}, fmt.Errorf("transaction.service.Update: %w", err)
	}
	return updated, nil
}

// List returns transactions for the given month, applying filters.
// If input.UserID is set and differs from the caller, both must be in the same couple.
func (s *Service) List(ctx context.Context, callerID string, input ListInput) ([]Transaction, error) {
	if input.Year == 0 || input.Month == 0 {
		return nil, &ValidationError{Field: "year/month", Message: "year and month are required"}
	}
	if input.Month < 1 || input.Month > 12 {
		return nil, &ValidationError{Field: "month", Message: "must be between 1 and 12"}
	}
	if input.Direction != nil && *input.Direction != "in" && *input.Direction != "out" {
		return nil, &ValidationError{Field: "direction", Message: `must be "in" or "out"`}
	}

	targetID := callerID
	if input.UserID != nil && *input.UserID != callerID {
		same, err := s.repo.sameCouple(ctx, callerID, *input.UserID)
		if err != nil {
			return nil, fmt.Errorf("transaction.service.List: %w", err)
		}
		if !same {
			return nil, ErrForbidden
		}
		targetID = *input.UserID
	}

	input.UserID = nil // already resolved into targetID
	txs, err := s.repo.list(ctx, targetID, &input)
	if err != nil {
		return nil, fmt.Errorf("transaction.service.List: %w", err)
	}
	return txs, nil
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
