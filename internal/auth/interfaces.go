package auth

import "context"

// repository defines the data access contract for the auth domain.
// Keeping it unexported restricts its use to this package.
type repository interface {
	createUser(ctx context.Context, p createUserParams) (userRow, error)
	saveRefreshToken(ctx context.Context, userID, tokenHash string) error
	findUserByEmail(ctx context.Context, email string) (loginUserRow, error)
	findUserByID(ctx context.Context, userID string) (userRow, error)
	findRefreshToken(ctx context.Context, tokenHash string) (refreshTokenRow, error)
	revokeRefreshToken(ctx context.Context, tokenID string) error
	revokeRefreshTokenByHash(ctx context.Context, userID, tokenHash string) error
	revokeAllUserTokens(ctx context.Context, userID string) error
	savePasswordResetToken(ctx context.Context, userID, tokenHash string) error
	findPasswordResetToken(ctx context.Context, tokenHash string) (passwordResetTokenRow, error)
	markPasswordResetTokenUsed(ctx context.Context, tokenID string) error
	updateUserPassword(ctx context.Context, userID, passwordHash string) error
}
