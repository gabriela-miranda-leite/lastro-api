package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the user domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers routes on the given router group (protected routes).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/users/me", h.me)
	rg.PUT("/users/me", h.updateMe)
	rg.GET("/users/me/export", h.export)
}

func (h *Handler) updateMe(c *gin.Context) {
	var input UpdateInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	resp, err := h.service.Update(c.Request.Context(), userID, input)
	if err != nil {
		var valErr *ValidationError
		switch {
		case errors.As(err, &valErr):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": valErr.Field + ": " + valErr.Message})
		case errors.Is(err, ErrNoFieldsToUpdate):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "no fields to update"})
		case errors.Is(err, ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) export(c *gin.Context) {
	userID := c.GetString("user_id")

	data, err := h.service.Export(c.Request.Context(), userID, c.ClientIP())
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	filename := "export-" + userID + ".json"
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.JSON(http.StatusOK, data)
}

func (h *Handler) me(c *gin.Context) {
	userID := c.GetString("user_id")

	resp, err := h.service.Me(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
