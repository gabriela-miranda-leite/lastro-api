package audit

import (
	"context"
	"fmt"
)

// Service contains business logic for the audit domain.
type Service struct {
	repo *Repository
}

// NewService creates a new Service.
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// List returns a paginated list of audit log entries for the user.
func (s *Service) List(ctx context.Context, userID string, input ListInput) (ListResponse, error) {
	entries, total, err := s.repo.list(ctx, userID, input)
	if err != nil {
		return ListResponse{}, fmt.Errorf("audit.service.List: %w", err)
	}
	return ListResponse{
		Data:   entries,
		Total:  total,
		Limit:  input.Limit,
		Offset: input.Offset,
	}, nil
}
