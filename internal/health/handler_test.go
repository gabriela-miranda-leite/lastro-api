package health_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/gabriela-miranda-leite/lastro-api/internal/health"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

func TestHealth_DBOk(t *testing.T) {
	db := testhelpers.SetupTestDB(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", health.NewHandler(db).Check)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"ok"`)
	assert.Contains(t, w.Body.String(), `"db":"ok"`)
}

func TestHealth_DBClosed(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	// Close the DB before the handler runs so PingContext fails.
	_ = db.Close()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", health.NewHandler(db).Check)

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/health", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"degraded"`)
}
