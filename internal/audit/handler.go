package audit

import "github.com/gin-gonic/gin"

// Handler handles HTTP requests for the audit domain.
type Handler struct {
	service *Service
}

// NewHandler creates a new Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers routes on the given router group.
func (h *Handler) RegisterRoutes(_ *gin.RouterGroup) {}
