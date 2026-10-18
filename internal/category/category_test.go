package category_test

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gabriela-miranda-leite/lastro-api/internal/category"
)

func TestCategory_Constructors(t *testing.T) {
	svc := &mockService{}
	h := category.NewHandlerWithService(svc)
	assert.NotNil(t, h)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	assert.NotPanics(t, func() {
		h.RegisterRoutes(r.Group("/v1"))
	})
}

type mockService struct{}

func (m *mockService) List(_ context.Context, _ string) ([]category.Category, error) {
	return nil, nil
}
func (m *mockService) Create(_ context.Context, _ string, _ category.CreateInput) (category.Category, error) {
	return category.Category{}, nil
}
func (m *mockService) Update(_ context.Context, _, _ string, _ category.UpdateInput) (category.Category, error) {
	return category.Category{}, nil
}
func (m *mockService) Delete(_ context.Context, _, _ string) error { return nil }
