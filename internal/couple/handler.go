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
	rg.POST("/couples/join", h.join)
}

func (h *Handler) join(c *gin.Context) {
	var input JoinInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		return
	}
	joinerID := c.GetString("user_id")

	resp, err := h.service.Join(c.Request.Context(), joinerID, input.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrInviteNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "invite not found"})
		case errors.Is(err, ErrInviteExpired):
			c.JSON(http.StatusGone, gin.H{"error": "invite expired or already used"})
		case errors.Is(err, ErrSelfJoin):
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot accept own invite"})
		case errors.Is(err, ErrAlreadyInCouple):
			c.JSON(http.StatusConflict, gin.H{"error": "user already belongs to a couple"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, resp)
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
