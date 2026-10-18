package category_test

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gabriela-miranda-leite/lastro-api/internal/category"
)

func TestCategory_Constructors(t *testing.T) {
	repo := category.NewRepository()
	assert.NotNil(t, repo)

	svc := category.NewService(repo)
	assert.NotNil(t, svc)

	h := category.NewHandler(svc)
	assert.NotNil(t, h)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	// RegisterRoutes is a no-op for now; must not panic
	assert.NotPanics(t, func() {
		h.RegisterRoutes(r.Group("/v1"))
	})
}
