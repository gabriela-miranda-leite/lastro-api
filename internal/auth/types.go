package auth

// RegisterInput is the request body for POST /auth/register.
type RegisterInput struct {
	Name     string `json:"name"     binding:"required"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginInput is the request body for POST /auth/login.
type LoginInput struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// UserResponse is the public user representation returned in responses.
type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// AuthResponse is returned after a successful register or login.
type AuthResponse struct {
	User         UserResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
}

// RefreshInput is the request body for POST /auth/refresh.
type RefreshInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// LogoutInput is the request body for POST /auth/logout.
type LogoutInput struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// ForgotPasswordInput is the request body for POST /auth/forgot-password.
type ForgotPasswordInput struct {
	Email string `json:"email" binding:"required,email"`
}

// ResetPasswordInput is the request body for POST /auth/reset-password.
type ResetPasswordInput struct {
	Token       string `json:"token"        binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}
