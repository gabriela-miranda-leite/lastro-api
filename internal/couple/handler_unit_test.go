package couple_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gabriela-miranda-leite/lastro-api/internal/couple"
)

// setupHandlerRouter wires up a minimal router with a fake authenticated user_id.
func setupHandlerRouter(svc *mockService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user_id", "user-test")
		c.Next()
	})
	h := couple.NewHandlerWithService(svc)
	h.RegisterRoutes(r.Group("/v1"))
	return r
}

// --- POST /couples/invite ---

func TestHandler_Invite_InternalError(t *testing.T) {
	r := setupHandlerRouter(&mockService{
		inviteFn: func(_ context.Context, _ string) (couple.InviteResponse, error) {
			return couple.InviteResponse{}, errInternal
		},
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/couples/invite", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

// --- POST /couples/join ---

func TestHandler_Join_InternalError(t *testing.T) {
	r := setupHandlerRouter(&mockService{
		joinFn: func(_ context.Context, _, _ string) (couple.JoinResponse, error) {
			return couple.JoinResponse{}, errInternal
		},
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/couples/join",
		strings.NewReader(`{"code":"ABCDEF"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

// --- DELETE /couples ---

func TestHandler_Unlink_InternalError(t *testing.T) {
	r := setupHandlerRouter(&mockService{
		unlinkFn: func(_ context.Context, _ string) error {
			return errInternal
		},
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/couples", strings.NewReader(""))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}
