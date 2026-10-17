package auth

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_Register(t *testing.T) {
	tests := []struct {
		name      string
		input     RegisterInput
		setupRepo func(*mockRepository)
		wantErr   error
	}{
		{
			name:    "password too short",
			input:   RegisterInput{Name: "Ana", Email: "ana@email.com", Password: "Ab1"},
			wantErr: ErrWeakPassword,
		},
		{
			name:    "password without uppercase",
			input:   RegisterInput{Name: "Ana", Email: "ana@email.com", Password: "senha123"},
			wantErr: ErrWeakPassword,
		},
		{
			name:    "password without digit",
			input:   RegisterInput{Name: "Ana", Email: "ana@email.com", Password: "SenhaSemNumero"},
			wantErr: ErrWeakPassword,
		},
		{
			name:  "email already in use",
			input: RegisterInput{Name: "Ana", Email: "dup@email.com", Password: "Senha123"},
			setupRepo: func(r *mockRepository) {
				r.createUserFn = func(_ context.Context, _ createUserParams) (userRow, error) {
					return userRow{}, ErrEmailConflict
				}
			},
			wantErr: ErrEmailConflict,
		},
		{
			name:  "success returns access and refresh tokens",
			input: RegisterInput{Name: "Ana", Email: "ana@email.com", Password: "Senha123"},
			setupRepo: func(r *mockRepository) {
				r.createUserFn = func(_ context.Context, p createUserParams) (userRow, error) {
					return userRow{ID: "user-1", Name: p.Name, Email: p.Email}, nil
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := &Service{repo: repo, jwtSecret: []byte("secret")}

			resp, err := svc.Register(context.Background(), tt.input)

			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, resp.AccessToken)
			assert.NotEmpty(t, resp.RefreshToken)
			assert.Equal(t, tt.input.Email, resp.User.Email)
		})
	}
}

func TestService_Login(t *testing.T) {
	const validHash = "$2a$12$LQv3c1yqBWVHxkd0LHAkCOYz6TtxMQJqhN8/LewdBPj4J/HS1Jvfq" // "Senha123" with cost 12

	tests := []struct {
		name      string
		input     LoginInput
		setupRepo func(*mockRepository)
		wantErr   error
	}{
		{
			name:  "user not found returns invalid credentials",
			input: LoginInput{Email: "nao@existe.com", Password: "Senha123"},
			setupRepo: func(r *mockRepository) {
				r.findUserByEmailFn = func(_ context.Context, _ string) (loginUserRow, error) {
					return loginUserRow{}, ErrInvalidCredentials
				}
			},
			wantErr: ErrInvalidCredentials,
		},
		{
			name:  "wrong password returns invalid credentials",
			input: LoginInput{Email: "ana@email.com", Password: "SenhaErrada1"},
			setupRepo: func(r *mockRepository) {
				r.findUserByEmailFn = func(_ context.Context, _ string) (loginUserRow, error) {
					return loginUserRow{ID: "u1", Email: "ana@email.com", PasswordHash: validHash}, nil
				}
			},
			wantErr: ErrInvalidCredentials,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			if tt.setupRepo != nil {
				tt.setupRepo(repo)
			}
			svc := &Service{repo: repo, jwtSecret: []byte("secret")}

			_, err := svc.Login(context.Background(), tt.input)

			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestService_Logout(t *testing.T) {
	t.Run("logout succeeds silently for any token", func(t *testing.T) {
		repo := &mockRepository{}
		svc := &Service{repo: repo, jwtSecret: []byte("secret")}
		err := svc.Logout(context.Background(), "user-1", "any-raw-token")
		assert.NoError(t, err)
	})
}
