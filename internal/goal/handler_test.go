package goal_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
	"github.com/gabriela-miranda-leite/lastro-api/internal/goal"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

const testJWTSecret = "test-secret"

func setupRouter(db *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), testJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))

	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, testJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)

	goal.NewHandler(goal.NewService(goal.NewRepository(db))).RegisterRoutes(protected)

	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func registerAndLogin(t *testing.T, r *gin.Engine, email, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"Test","email":%q,"password":%q}`, email, password)
	w := doReq(t, r, http.MethodPost, "/v1/auth/register", body, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["access_token"].(string)
}

// --- Integration ---

func TestIntegration_Goal(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("POST creates goal", func(t *testing.T) {
		token := registerAndLogin(t, r, "goal_create@test.com", "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/goals",
			`{"year":2026,"month":10,"goal_amount":1000}`, token)
		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, float64(2026), resp["year"])
		assert.Equal(t, float64(10), resp["month"])
		assert.Equal(t, "1000.00", resp["goal_amount"])
		assert.Equal(t, "0.00", resp["saved_amount"])
	})

	t.Run("POST duplicate returns 409", func(t *testing.T) {
		token := registerAndLogin(t, r, "goal_dup@test.com", "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, token)
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, token)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("POST different months allowed", func(t *testing.T) {
		token := registerAndLogin(t, r, "goal_months@test.com", "Senha123")
		w1 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, token)
		w2 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":11,"goal_amount":600}`, token)
		assert.Equal(t, http.StatusCreated, w1.Code)
		assert.Equal(t, http.StatusCreated, w2.Code)
	})

	t.Run("POST same month different users allowed", func(t *testing.T) {
		tokenA := registerAndLogin(t, r, "goal_userA@test.com", "Senha123")
		tokenB := registerAndLogin(t, r, "goal_userB@test.com", "Senha123")
		w1 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, tokenA)
		w2 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, tokenB)
		assert.Equal(t, http.StatusCreated, w1.Code)
		assert.Equal(t, http.StatusCreated, w2.Code)
	})

	t.Run("POST goal_amount zero returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, "goal_amt@test.com", "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":0}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("POST invalid month returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, "goal_month@test.com", "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":13,"goal_amount":100}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("POST without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":100}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Handler unit test for 500 path ---

type mockSvc struct{ err error }

func (m *mockSvc) Create(_ context.Context, _ string, _ goal.CreateInput) (goal.Goal, error) {
	return goal.Goal{}, m.err
}

func TestHandler_Create_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := goal.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/goals",
		strings.NewReader(`{"year":2026,"month":10,"goal_amount":100}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
