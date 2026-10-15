package dashboard

import "github.com/gin-gonic/gin"

// Handler handles HTTP requests for the dashboard domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(_ *gin.RouterGroup) {}
