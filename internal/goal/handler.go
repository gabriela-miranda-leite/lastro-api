package goal

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type goalService interface {
	List(ctx context.Context, userID string, input ListInput) ([]GoalWithStatus, error)
	Create(ctx context.Context, userID string, input CreateInput) (Goal, error)
	Update(ctx context.Context, userID, id string, input UpdateInput) (Goal, error)
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
	rg.GET("/goals", h.list)
	rg.POST("/goals", h.create)
	rg.PUT("/goals/:id", h.update)
}

func (h *Handler) update(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("user_id")

	var input UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	g, err := h.service.Update(c.Request.Context(), userID, id, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Message, "field": valErr.Field})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "goal not found"})
		case errors.Is(err, ErrForbidden):
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, g)
}

func (h *Handler) list(c *gin.Context) {
	userID := c.GetString("user_id")

	var input ListInput
	if yearStr := c.Query("year"); yearStr != "" {
		var year int
		if _, err := fmt.Sscanf(yearStr, "%d", &year); err != nil || year <= 0 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid year", "field": "year"})
			return
		}
		input.Year = &year
	}
	if monthStr := c.Query("month"); monthStr != "" {
		if input.Year == nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "month requires year", "field": "month"})
			return
		}
		var month int
		if _, err := fmt.Sscanf(monthStr, "%d", &month); err != nil || month < 1 || month > 12 {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid month", "field": "month"})
			return
		}
		input.Month = &month
	}

	goals, err := h.service.List(c.Request.Context(), userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if goals == nil {
		goals = []GoalWithStatus{}
	}
	c.JSON(http.StatusOK, goals)
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
