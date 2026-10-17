package transaction

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type transactionService interface {
	Create(ctx context.Context, userID string, input *CreateInput) (Transaction, bool, error)
	List(ctx context.Context, callerID string, input ListInput) ([]Transaction, error)
	Update(ctx context.Context, userID, id string, input UpdateInput) (Transaction, error)
	Delete(ctx context.Context, userID, id string) error
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
	rg.GET("/transactions", h.list)
	rg.PUT("/transactions/:id", h.update)
	rg.DELETE("/transactions/:id", h.delete)
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

func (h *Handler) list(c *gin.Context) {
	yearStr := c.Query("year")
	monthStr := c.Query("month")
	if yearStr == "" || monthStr == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "year and month are required"})
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "year must be a number", "field": "year"})
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "month must be a number", "field": "month"})
		return
	}

	input := ListInput{Year: year, Month: month}
	if v := c.Query("category_id"); v != "" {
		input.CategoryID = &v
	}
	if v := c.Query("direction"); v != "" {
		input.Direction = &v
	}
	if v := c.Query("user_id"); v != "" {
		input.UserID = &v
	}

	callerID := c.GetString("user_id")
	txs, err := h.service.List(c.Request.Context(), callerID, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "cannot access transactions of this user"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	if txs == nil {
		txs = []Transaction{}
	}
	c.JSON(http.StatusOK, txs)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var input UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	tx, err := h.service.Update(c.Request.Context(), userID, id, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "not the owner of this transaction"})
		case errors.Is(err, ErrCategoryForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "category belongs to another user"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, tx)
}

func (h *Handler) delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	err := h.service.Delete(c.Request.Context(), userID, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "transaction not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "not the owner of this transaction"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}
