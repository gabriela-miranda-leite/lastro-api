package auth

import "context"

// mockRepository is a manual mock of the repository interface for unit tests.
// Configure only the functions your test case needs — unconfigured methods are no-ops.
type mockRepository struct {
	createUserFn                 func(ctx context.Context, p createUserParams) (userRow, error)
	saveRefreshTokenFn           func(ctx context.Context, userID, tokenHash string) error
	findUserByEmailFn            func(ctx context.Context, email string) (loginUserRow, error)
	findUserByIDFn               func(ctx context.Context, userID string) (userRow, error)
	findRefreshTokenFn           func(ctx context.Context, tokenHash string) (refreshTokenRow, error)
	revokeRefreshTokenFn         func(ctx context.Context, tokenID string) error
	revokeRefreshTokenByHashFn   func(ctx context.Context, userID, tokenHash string) error
	revokeAllUserTokensFn        func(ctx context.Context, userID string) error
	savePasswordResetTokenFn     func(ctx context.Context, userID, tokenHash string) error
	findPasswordResetTokenFn     func(ctx context.Context, tokenHash string) (passwordResetTokenRow, error)
	markPasswordResetTokenUsedFn func(ctx context.Context, tokenID string) error
	updateUserPasswordFn         func(ctx context.Context, userID, passwordHash string) error
}

func (m *mockRepository) createUser(ctx context.Context, p createUserParams) (userRow, error) {
	if m.createUserFn != nil {
		return m.createUserFn(ctx, p)
	}
	return userRow{}, nil
}

func (m *mockRepository) saveRefreshToken(ctx context.Context, userID, tokenHash string) error {
	if m.saveRefreshTokenFn != nil {
		return m.saveRefreshTokenFn(ctx, userID, tokenHash)
	}
	return nil
}

func (m *mockRepository) findUserByEmail(ctx context.Context, email string) (loginUserRow, error) {
	if m.findUserByEmailFn != nil {
		return m.findUserByEmailFn(ctx, email)
	}
	return loginUserRow{}, nil
}

func (m *mockRepository) findUserByID(ctx context.Context, userID string) (userRow, error) {
	if m.findUserByIDFn != nil {
		return m.findUserByIDFn(ctx, userID)
	}
	return userRow{}, nil
}

func (m *mockRepository) findRefreshToken(ctx context.Context, tokenHash string) (refreshTokenRow, error) {
	if m.findRefreshTokenFn != nil {
		return m.findRefreshTokenFn(ctx, tokenHash)
	}
	return refreshTokenRow{}, nil
}

func (m *mockRepository) revokeRefreshToken(ctx context.Context, tokenID string) error {
	if m.revokeRefreshTokenFn != nil {
		return m.revokeRefreshTokenFn(ctx, tokenID)
	}
	return nil
}

func (m *mockRepository) revokeRefreshTokenByHash(ctx context.Context, userID, tokenHash string) error {
	if m.revokeRefreshTokenByHashFn != nil {
		return m.revokeRefreshTokenByHashFn(ctx, userID, tokenHash)
	}
	return nil
}

func (m *mockRepository) revokeAllUserTokens(ctx context.Context, userID string) error {
	if m.revokeAllUserTokensFn != nil {
		return m.revokeAllUserTokensFn(ctx, userID)
	}
	return nil
}

func (m *mockRepository) savePasswordResetToken(ctx context.Context, userID, tokenHash string) error {
	if m.savePasswordResetTokenFn != nil {
		return m.savePasswordResetTokenFn(ctx, userID, tokenHash)
	}
	return nil
}

func (m *mockRepository) findPasswordResetToken(ctx context.Context, tokenHash string) (passwordResetTokenRow, error) {
	if m.findPasswordResetTokenFn != nil {
		return m.findPasswordResetTokenFn(ctx, tokenHash)
	}
	return passwordResetTokenRow{ID: "token-1", UserID: "user-1"}, nil
}

func (m *mockRepository) markPasswordResetTokenUsed(ctx context.Context, tokenID string) error {
	if m.markPasswordResetTokenUsedFn != nil {
		return m.markPasswordResetTokenUsedFn(ctx, tokenID)
	}
	return nil
}

func (m *mockRepository) updateUserPassword(ctx context.Context, userID, passwordHash string) error {
	if m.updateUserPasswordFn != nil {
		return m.updateUserPasswordFn(ctx, userID, passwordHash)
	}
	return nil
}
