package user

import (
	"context"
	"fmt"
	"unicode"

	"golang.org/x/crypto/bcrypt"
)

func (s *Service) ChangePassword(ctx context.Context, userID string, input ChangePasswordInput) error {
	if len(input.NewPassword) < 8 {
		return &ValidationError{Field: "new_password", Message: "must be at least 8 characters"}
	}

	hasLetter, hasDigit := false, false
	for _, ch := range input.NewPassword {
		if unicode.IsLetter(ch) {
			hasLetter = true
		}
		if unicode.IsDigit(ch) {
			hasDigit = true
		}
	}
	if !hasLetter || !hasDigit {
		return &ValidationError{Field: "new_password", Message: "must contain at least one letter and one number"}
	}

	hash, err := s.repo.findPasswordHash(ctx, userID)
	if err != nil {
		return fmt.Errorf("user.service.ChangePassword: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.CurrentPassword)); err != nil {
		return ErrInvalidPassword
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(input.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("user.service.ChangePassword: hash: %w", err)
	}

	if err := s.repo.updatePasswordHash(ctx, userID, string(newHash)); err != nil {
		return fmt.Errorf("user.service.ChangePassword: %w", err)
	}

	return nil
}

func (r *Repository) updatePasswordHash(ctx context.Context, userID, newHash string) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash = $1 WHERE id = $2 AND deleted_at IS NULL`,
		newHash, userID,
	)
	if err != nil {
		return fmt.Errorf("user.repository.updatePasswordHash: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("user.repository.updatePasswordHash: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}

	return nil
}
