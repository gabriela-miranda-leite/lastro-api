package recurring

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type recurringService interface {
	List(ctx context.Context, userID string) ([]Recurring, error)
	Create(ctx context.Context, userID string, input CreateInput) (Recurring, error)
	Update(ctx context.Context, userID, id string, input UpdateInput) (Recurring, error)
	Delete(ctx context.Context, userID, id string) error
	UpsertOverride(ctx context.Context, userID, recurringID string, input OverrideInput) (RecurringOverride, error)
}

// Handler handles HTTP requests for the recurring domain.
type Handler struct {
	service recurringService
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// NewHandlerWithService creates a Handler from any recurringService — used in tests.
func NewHandlerWithService(svc recurringService) *Handler {
	return &Handler{service: svc}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/recurring", h.list)
	rg.POST("/recurring", h.create)
	rg.PUT("/recurring/:id", h.update)
	rg.DELETE("/recurring/:id", h.delete)
	rg.PUT("/recurring/:id/override", h.upsertOverride)
}

func (h *Handler) list(c *gin.Context) {
	userID := c.GetString("user_id")
	recs, err := h.service.List(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if recs == nil {
		recs = []Recurring{}
	}
	c.JSON(http.StatusOK, recs)
}

func (h *Handler) create(c *gin.Context) {
	var input CreateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetString("user_id")
	rec, err := h.service.Create(c.Request.Context(), userID, input)
	if err != nil {
		var valErr *ValidationError
		if errors.As(err, &valErr) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusCreated, rec)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	var input UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetString("user_id")
	rec, err := h.service.Update(c.Request.Context(), userID, id, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "recurring expense not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "not the owner of this recurring expense"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, rec)
}

func (h *Handler) delete(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")
	err := h.service.Delete(c.Request.Context(), userID, id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "recurring expense not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "not the owner of this recurring expense"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) upsertOverride(c *gin.Context) {
	id := c.Param("id")
	var input OverrideInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetString("user_id")
	o, err := h.service.UpsertOverride(c.Request.Context(), userID, id, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "recurring expense not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "not the owner of this recurring expense"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, o)
}
