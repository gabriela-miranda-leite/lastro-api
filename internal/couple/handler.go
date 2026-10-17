package couple

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the couple domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/couples/invite", h.invite)
}

func (h *Handler) invite(c *gin.Context) {
	userID := c.GetString("user_id")

	resp, err := h.service.Invite(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrAlreadyInCouple) {
			c.JSON(http.StatusConflict, gin.H{"error": "user already belongs to a couple"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}
