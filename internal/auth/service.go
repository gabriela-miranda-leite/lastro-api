package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrWeakPassword = errors.New("password must be at least 8 characters, contain one uppercase letter and one number")
)

// Service contains business logic for the auth domain.
type Service struct {
	repo      repository
	jwtSecret []byte
}

// NewService creates a new Service.
func NewService(repo *Repository, jwtSecret string) *Service {
	return &Service{repo: repo, jwtSecret: []byte(jwtSecret)}
}

func (s *Service) Register(ctx context.Context, input RegisterInput) (AuthResponse, error) {
	if err := validatePassword(input.Password); err != nil {
		return AuthResponse{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Register: hash: %w", err)
	}

	user, err := s.repo.createUser(ctx, createUserParams{
		Name:         input.Name,
		Email:        input.Email,
		PasswordHash: string(hash),
	})
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Register: %w", err)
	}

	accessToken, err := s.generateAccessToken(user.ID)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Register: %w", err)
	}

	rawRefresh, refreshHash, err := generateRefreshToken()
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Register: %w", err)
	}

	if err := s.repo.saveRefreshToken(ctx, user.ID, refreshHash); err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Register: %w", err)
	}

	return AuthResponse{
		User:         UserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	}, nil
}

func (s *Service) Login(ctx context.Context, input LoginInput) (AuthResponse, error) {
	user, err := s.repo.findUserByEmail(ctx, input.Email)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Login: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return AuthResponse{}, ErrInvalidCredentials
	}

	accessToken, err := s.generateAccessToken(user.ID)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Login: %w", err)
	}

	rawRefresh, refreshHash, err := generateRefreshToken()
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Login: %w", err)
	}

	if err := s.repo.saveRefreshToken(ctx, user.ID, refreshHash); err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Login: %w", err)
	}

	return AuthResponse{
		User:         UserResponse{ID: user.ID, Name: user.Name, Email: user.Email},
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	}, nil
}

func (s *Service) Refresh(ctx context.Context, rawToken string) (AuthResponse, error) {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	tokenRow, err := s.repo.findRefreshToken(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, ErrTokenRevoked) {
			// reuse detected — revoke all sessions for this user
			_ = s.repo.revokeAllUserTokens(ctx, tokenRow.UserID)
			return AuthResponse{}, fmt.Errorf("auth.service.Refresh: %w", ErrInvalidCredentials)
		}
		return AuthResponse{}, fmt.Errorf("auth.service.Refresh: %w", err)
	}

	if err := s.repo.revokeRefreshToken(ctx, tokenRow.ID); err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Refresh: %w", err)
	}

	user, err := s.repo.findUserByID(ctx, tokenRow.UserID)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Refresh: %w", err)
	}

	accessToken, err := s.generateAccessToken(user.ID)
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Refresh: %w", err)
	}

	rawRefresh, refreshHash, err := generateRefreshToken()
	if err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Refresh: %w", err)
	}

	if err := s.repo.saveRefreshToken(ctx, user.ID, refreshHash); err != nil {
		return AuthResponse{}, fmt.Errorf("auth.service.Refresh: %w", err)
	}

	return AuthResponse{
		User:         UserResponse(user),
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
	}, nil
}

func (s *Service) Logout(ctx context.Context, userID, rawToken string) error {
	sum := sha256.Sum256([]byte(rawToken))
	tokenHash := hex.EncodeToString(sum[:])

	if err := s.repo.revokeRefreshTokenByHash(ctx, userID, tokenHash); err != nil {
		return fmt.Errorf("auth.service.Logout: %w", err)
	}
	return nil
}

func (s *Service) LogoutAll(ctx context.Context, userID string) error {
	if err := s.repo.revokeAllUserTokens(ctx, userID); err != nil {
		return fmt.Errorf("auth.service.LogoutAll: %w", err)
	}
	return nil
}

func (s *Service) generateAccessToken(userID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("generateAccessToken: %w", err)
	}
	return signed, nil
}

func generateRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generateRefreshToken: %w", err)
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}

func validatePassword(p string) error {
	if len(p) < 8 {
		return ErrWeakPassword
	}
	var hasUpper, hasDigit bool
	for _, r := range p {
		if unicode.IsUpper(r) {
			hasUpper = true
		}
		if unicode.IsDigit(r) {
			hasDigit = true
		}
	}
	if !hasUpper || !hasDigit {
		return ErrWeakPassword
	}
	return nil
}
