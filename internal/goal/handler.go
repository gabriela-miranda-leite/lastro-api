package goal

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type goalService interface {
	Create(ctx context.Context, userID string, input CreateInput) (Goal, error)
}

// Handler handles HTTP requests for the goal domain.
type Handler struct {
	service goalService
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// NewHandlerWithService creates a Handler from any goalService — used in tests.
func NewHandlerWithService(svc goalService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/goals", h.create)
}

func (h *Handler) create(c *gin.Context) {
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetString("user_id")
	g, err := h.service.Create(c.Request.Context(), userID, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "goal already exists for this month"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusCreated, g)
}
