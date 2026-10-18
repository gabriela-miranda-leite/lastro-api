package dashboard

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the dashboard domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/dashboard/:year/:month", h.get)
}

func (h *Handler) get(c *gin.Context) {
	year, err := strconv.Atoi(c.Param("year"))
	if err != nil || year <= 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid year"})
		return
	}
	month, err := strconv.Atoi(c.Param("month"))
	if err != nil || month < 1 || month > 12 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "invalid month"})
		return
	}

	userID := c.GetString("user_id")
	resp, err := h.service.Get(c.Request.Context(), userID, year, month)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}
