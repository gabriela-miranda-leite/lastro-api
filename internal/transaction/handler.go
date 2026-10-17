package transaction

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type transactionService interface {
	Create(ctx context.Context, userID string, input *CreateInput) (Transaction, bool, error)
}

// Handler handles HTTP requests for the transaction domain.
type Handler struct {
	service transactionService
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// NewHandlerWithService creates a Handler from any transactionService — used in tests.
func NewHandlerWithService(svc transactionService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/transactions", h.create)
}

func (h *Handler) create(c *gin.Context) {
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")

	tx, existed, err := h.service.Create(c.Request.Context(), userID, &input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrSharedWithoutCouple):
			c.JSON(http.StatusBadRequest, gin.H{"error": "is_shared requires a linked couple"})
		case errors.Is(err, ErrCategoryForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "category belongs to another user"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	if existed {
		c.JSON(http.StatusOK, tx)
		return
	}
	c.JSON(http.StatusCreated, tx)
}
