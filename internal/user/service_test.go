package user

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func ptr[T any](v T) *T { return &v }

func TestService_Update(t *testing.T) {
	tests := []struct {
		name      string
		input     UpdateInput
		wantField string
		wantMsg   string
	}{
		{
			name:      "empty name",
			input:     UpdateInput{Name: ptr("")},
			wantField: "name",
			wantMsg:   "cannot be empty",
		},
		{
			name:      "name too long",
			input:     UpdateInput{Name: ptr(strings.Repeat("a", 151))},
			wantField: "name",
			wantMsg:   "150 characters",
		},
		{
			name:      "monthly_income zero",
			input:     UpdateInput{MonthlyIncome: ptr(0.0)},
			wantField: "monthly_income",
			wantMsg:   "greater than zero",
		},
		{
			name:      "monthly_income negative",
			input:     UpdateInput{MonthlyIncome: ptr(-1.0)},
			wantField: "monthly_income",
			wantMsg:   "greater than zero",
		},
		{
			name:      "saving_goal_pct above 100",
			input:     UpdateInput{SavingGoalPct: ptr(100.01)},
			wantField: "saving_goal_pct",
			wantMsg:   "between 0 and 100",
		},
		{
			name:      "saving_goal_pct negative",
			input:     UpdateInput{SavingGoalPct: ptr(-0.01)},
			wantField: "saving_goal_pct",
			wantMsg:   "between 0 and 100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &Service{repo: &mockRepository{}}

			_, err := svc.Update(context.Background(), "user-1", tt.input)

			require.Error(t, err)
			var valErr *ValidationError
			require.ErrorAs(t, err, &valErr)
			assert.Equal(t, tt.wantField, valErr.Field)
			assert.Contains(t, valErr.Message, tt.wantMsg)
		})
	}

	t.Run("valid update calls repo and returns updated profile", func(t *testing.T) {
		updated := MeResponse{ID: "user-1", Name: "Novo Nome", Email: "a@test.com"}
		repo := &mockRepository{
			findByIDFn: func(_ context.Context, _ string) (MeResponse, error) {
				return updated, nil
			},
		}
		svc := &Service{repo: repo}

		resp, err := svc.Update(context.Background(), "user-1", UpdateInput{Name: ptr("Novo Nome")})

		require.NoError(t, err)
		assert.Equal(t, "Novo Nome", resp.Name)
	})

	t.Run("boundary: saving_goal_pct exactly 0 and 100 are valid", func(t *testing.T) {
		svc := &Service{repo: &mockRepository{}}

		_, err := svc.Update(context.Background(), "user-1", UpdateInput{SavingGoalPct: ptr(0.0)})
		assert.NoError(t, err)

		_, err = svc.Update(context.Background(), "user-1", UpdateInput{SavingGoalPct: ptr(100.0)})
		assert.NoError(t, err)
	})
}

func TestService_Delete(t *testing.T) {
	validHash, _ := bcrypt.GenerateFromPassword([]byte("Senha123"), bcrypt.MinCost)

	t.Run("wrong password returns ErrInvalidPassword", func(t *testing.T) {
		repo := &mockRepository{
			findPasswordHashFn: func(_ context.Context, _ string) (string, error) {
				return string(validHash), nil
			},
		}
		svc := &Service{repo: repo}

		err := svc.Delete(context.Background(), "user-1", DeleteInput{Password: "SenhaErrada1"})

		assert.ErrorIs(t, err, ErrInvalidPassword)
	})

	t.Run("correct password calls softDelete", func(t *testing.T) {
		softDeleteCalled := false
		repo := &mockRepository{
			findPasswordHashFn: func(_ context.Context, _ string) (string, error) {
				return string(validHash), nil
			},
			softDeleteFn: func(_ context.Context, _, _ string) error {
				softDeleteCalled = true
				return nil
			},
		}
		svc := &Service{repo: repo}

		err := svc.Delete(context.Background(), "user-1", DeleteInput{Password: "Senha123"})

		require.NoError(t, err)
		assert.True(t, softDeleteCalled)
	})

	t.Run("user not found returns ErrNotFound", func(t *testing.T) {
		repo := &mockRepository{
			findPasswordHashFn: func(_ context.Context, _ string) (string, error) {
				return "", ErrNotFound
			},
		}
		svc := &Service{repo: repo}

		err := svc.Delete(context.Background(), "ghost", DeleteInput{Password: "Senha123"})

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestService_Me(t *testing.T) {
	t.Run("returns user profile", func(t *testing.T) {
		repo := &mockRepository{
			findByIDFn: func(_ context.Context, userID string) (MeResponse, error) {
				return MeResponse{ID: userID, Name: "Test"}, nil
			},
		}
		svc := &Service{repo: repo}

		resp, err := svc.Me(context.Background(), "user-1")

		require.NoError(t, err)
		assert.Equal(t, "user-1", resp.ID)
		assert.Equal(t, "Test", resp.Name)
	})

	t.Run("not found propagates ErrNotFound", func(t *testing.T) {
		repo := &mockRepository{
			findByIDFn: func(_ context.Context, _ string) (MeResponse, error) {
				return MeResponse{}, ErrNotFound
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Me(context.Background(), "ghost")

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestService_Export(t *testing.T) {
	t.Run("success returns export data and records audit log", func(t *testing.T) {
		auditCalled := false
		repo := &mockRepository{
			buildExportFn: func(_ context.Context, userID string) (ExportData, error) {
				return ExportData{User: exportUser{ID: userID}}, nil
			},
			insertAuditLogFn: func(_ context.Context, _, _, _ string) error {
				auditCalled = true
				return nil
			},
		}
		svc := &Service{repo: repo}

		data, err := svc.Export(context.Background(), "user-1", "127.0.0.1")

		require.NoError(t, err)
		assert.Equal(t, "user-1", data.User.ID)
		assert.True(t, auditCalled)
	})

	t.Run("not found propagates ErrNotFound", func(t *testing.T) {
		repo := &mockRepository{
			buildExportFn: func(_ context.Context, _ string) (ExportData, error) {
				return ExportData{}, ErrNotFound
			},
		}
		svc := &Service{repo: repo}

		_, err := svc.Export(context.Background(), "ghost", "127.0.0.1")

		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestValidationError_Error(t *testing.T) {
	e := &ValidationError{Field: "name", Message: "cannot be empty"}
	assert.Equal(t, "name: cannot be empty", e.Error())
}
