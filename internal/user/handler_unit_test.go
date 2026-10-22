package user

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockService struct {
	meFn             func(ctx context.Context, userID string) (MeResponse, error)
	exportFn         func(ctx context.Context, userID, ip string) (ExportData, error)
	updateFn         func(ctx context.Context, userID string, input UpdateInput) (MeResponse, error)
	deleteFn         func(ctx context.Context, userID string, input DeleteInput) error
	changePasswordFn func(ctx context.Context, userID string, input ChangePasswordInput) error
}

func (m *mockService) Me(ctx context.Context, userID string) (MeResponse, error) {
	if m.meFn != nil {
		return m.meFn(ctx, userID)
	}
	return MeResponse{ID: userID}, nil
}

func (m *mockService) Export(ctx context.Context, userID, ip string) (ExportData, error) {
	if m.exportFn != nil {
		return m.exportFn(ctx, userID, ip)
	}
	return ExportData{}, nil
}

func (m *mockService) Update(ctx context.Context, userID string, input UpdateInput) (MeResponse, error) {
	if m.updateFn != nil {
		return m.updateFn(ctx, userID, input)
	}
	return MeResponse{ID: userID}, nil
}

func (m *mockService) Delete(ctx context.Context, userID string, input DeleteInput) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, userID, input)
	}
	return nil
}

func (m *mockService) ChangePassword(ctx context.Context, userID string, input ChangePasswordInput) error {
	if m.changePasswordFn != nil {
		return m.changePasswordFn(ctx, userID, input)
	}
	return nil
}

func handlerRouter(svc userService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &Handler{service: svc}
	r.GET("/users/me", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.me(c)
	})
	r.GET("/users/me/export", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.export(c)
	})
	r.PUT("/users/me", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.updateMe(c)
	})
	r.DELETE("/users/me", func(c *gin.Context) {
		c.Set("user_id", "user-1")
		h.deleteMe(c)
	})
	return r
}

func do(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestHandlerMe_NotFound(t *testing.T) {
	svc := &mockService{meFn: func(_ context.Context, _ string) (MeResponse, error) {
		return MeResponse{}, ErrNotFound
	}}
	w := do(handlerRouter(svc), http.MethodGet, "/users/me", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerMe_InternalError(t *testing.T) {
	svc := &mockService{meFn: func(_ context.Context, _ string) (MeResponse, error) {
		return MeResponse{}, errors.New("db down")
	}}
	w := do(handlerRouter(svc), http.MethodGet, "/users/me", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlerExport_NotFound(t *testing.T) {
	svc := &mockService{exportFn: func(_ context.Context, _, _ string) (ExportData, error) {
		return ExportData{}, ErrNotFound
	}}
	w := do(handlerRouter(svc), http.MethodGet, "/users/me/export", "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerExport_InternalError(t *testing.T) {
	svc := &mockService{exportFn: func(_ context.Context, _, _ string) (ExportData, error) {
		return ExportData{}, errors.New("db down")
	}}
	w := do(handlerRouter(svc), http.MethodGet, "/users/me/export", "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlerUpdateMe_InternalError(t *testing.T) {
	svc := &mockService{updateFn: func(_ context.Context, _ string, _ UpdateInput) (MeResponse, error) {
		return MeResponse{}, errors.New("db down")
	}}
	w := do(handlerRouter(svc), http.MethodPut, "/users/me", `{"name":"X"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandlerUpdateMe_NotFound(t *testing.T) {
	svc := &mockService{updateFn: func(_ context.Context, _ string, _ UpdateInput) (MeResponse, error) {
		return MeResponse{}, ErrNotFound
	}}
	w := do(handlerRouter(svc), http.MethodPut, "/users/me", `{"name":"X"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerDeleteMe_NotFound(t *testing.T) {
	svc := &mockService{deleteFn: func(_ context.Context, _ string, _ DeleteInput) error {
		return ErrNotFound
	}}
	w := do(handlerRouter(svc), http.MethodDelete, "/users/me", `{"password":"Senha123"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlerDeleteMe_InternalError(t *testing.T) {
	svc := &mockService{deleteFn: func(_ context.Context, _ string, _ DeleteInput) error {
		return errors.New("db down")
	}}
	w := do(handlerRouter(svc), http.MethodDelete, "/users/me", `{"password":"Senha123"}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
