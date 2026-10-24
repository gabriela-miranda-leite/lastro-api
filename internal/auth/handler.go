package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the auth domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers public auth routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/register", h.register)
	rg.POST("/login", h.login)
	rg.POST("/refresh", h.refresh)
	rg.POST("/forgot-password", h.forgotPassword)
}

// RegisterProtectedRoutes registers auth routes that require a valid JWT.
func (h *Handler) RegisterProtectedRoutes(rg *gin.RouterGroup) {
	rg.POST("/auth/logout", h.logout)
	rg.POST("/auth/logout/all", h.logoutAll)
}

func (h *Handler) login(c *gin.Context) {
	var input LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.Login(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) refresh(c *gin.Context) {
	var input RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.Refresh(c.Request.Context(), input.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
			return
		}
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) logout(c *gin.Context) {
	var input LogoutInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if err := h.service.Logout(c.Request.Context(), userID, input.RefreshToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handler) logoutAll(c *gin.Context) {
	userID := c.GetString("user_id")
	if err := h.service.LogoutAll(c.Request.Context(), userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) forgotPassword(c *gin.Context) {
	var input ForgotPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	// Always return 200 regardless of whether email exists — prevents user enumeration.
	if err := h.service.ForgotPassword(c.Request.Context(), input); err != nil {
		slog.Warn("forgotPassword: internal error", "error", err)
	}
	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, you will receive a reset link"})
}

func (h *Handler) register(c *gin.Context) {
	var input RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.service.Register(c.Request.Context(), input)
	if err != nil {
		if errors.Is(err, ErrWeakPassword) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrEmailConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "email already in use"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}
