package category

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type categoryService interface {
	List(ctx context.Context, userID string) ([]Category, error)
	Create(ctx context.Context, userID string, input CreateInput) (Category, error)
	Update(ctx context.Context, userID, id string, input UpdateInput) (Category, error)
	Delete(ctx context.Context, userID, id string) error
}

// Handler handles HTTP requests for the category domain.
type Handler struct {
	service categoryService
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// NewHandlerWithService creates a Handler from any categoryService — used in tests.
func NewHandlerWithService(svc categoryService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/categories", h.list)
	rg.POST("/categories", h.create)
	rg.PUT("/categories/:id", h.update)
	rg.DELETE("/categories/:id", h.delete)
}

func (h *Handler) list(c *gin.Context) {
	userID := c.GetString("user_id")
	cats, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if cats == nil {
		cats = []Category{}
	}
	c.JSON(http.StatusOK, cats)
}

func (h *Handler) create(c *gin.Context) {
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetString("user_id")
	cat, err := h.service.Create(c.Request.Context(), userID, input)
	if err != nil {
		var valErr *ValidationError
		if errors.As(err, &valErr) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusCreated, cat)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	var input UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	cat, err := h.service.Update(c.Request.Context(), userID, id, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, cat)
}

func (h *Handler) delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	err := h.service.Delete(c.Request.Context(), userID, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "category not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusNoContent, nil)
}
