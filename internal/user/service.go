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

func (s *Service) Export(ctx context.Context, userID, ip string) (ExportData, error) {
	export, err := s.repo.buildExport(ctx, userID)
	if err != nil {
		return ExportData{}, fmt.Errorf("user.service.Export: %w", err)
	}

	if err := s.repo.insertAuditLog(ctx, userID, "export_data", ip); err != nil {
		return ExportData{}, fmt.Errorf("user.service.Export: %w", err)
	}

	return export, nil
}

func (s *Service) Update(ctx context.Context, userID string, input UpdateInput) (MeResponse, error) {
	if input.Name != nil {
		if *input.Name == "" {
			return MeResponse{}, &ValidationError{Field: "name", Message: "cannot be empty"}
		}
		if len(*input.Name) > 150 {
			return MeResponse{}, &ValidationError{Field: "name", Message: "must be 150 characters or fewer"}
		}
	}
	if input.MonthlyIncome != nil && *input.MonthlyIncome <= 0 {
		return MeResponse{}, &ValidationError{Field: "monthly_income", Message: "must be greater than zero"}
	}
	if input.SavingGoalPct != nil && (*input.SavingGoalPct < 0 || *input.SavingGoalPct > 100) {
		return MeResponse{}, &ValidationError{Field: "saving_goal_pct", Message: "must be between 0 and 100"}
	}

	if err := s.repo.update(ctx, userID, input); err != nil {
		return MeResponse{}, fmt.Errorf("user.service.Update: %w", err)
	}

	return s.Me(ctx, userID)
}
