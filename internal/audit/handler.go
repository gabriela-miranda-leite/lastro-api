package audit

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for the audit domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/audit", h.list)
	// Audit log is immutable — reject mutating methods explicitly.
	notAllowed := func(c *gin.Context) {
		c.Status(http.StatusMethodNotAllowed)
	}
	rg.PUT("/audit", notAllowed)
	rg.PATCH("/audit", notAllowed)
	rg.DELETE("/audit", notAllowed)
}

func (h *Handler) list(c *gin.Context) {
	limit, err := parsePositiveInt(c.DefaultQuery("limit", "20"), 1, 100)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "limit must be between 1 and 100"})
		return
	}
	offset, err := parseNonNegativeInt(c.DefaultQuery("offset", "0"))
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "offset must be a non-negative integer"})
		return
	}

	userID := c.GetString("user_id")
	resp, err := h.service.List(c.Request.Context(), userID, ListInput{Limit: limit, Offset: offset})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, resp)
}

func parsePositiveInt(s string, minVal, maxVal int) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v < minVal || v > maxVal {
		return 0, strconv.ErrRange
	}
	return v, nil
}

func parseNonNegativeInt(s string) (int, error) {
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return 0, strconv.ErrRange
	}
	return v, nil
}
