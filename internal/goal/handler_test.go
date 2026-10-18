package goal_test

import (
	"context"
	"crypto/md5" //nolint:gosec // used only for non-security email uniqueness
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

// uniqueEmail generates a unique email per sub-test to avoid collisions when
// tests share a database (e.g. CI with TEST_DATABASE_URL).
func uniqueEmail(t *testing.T, label string) string {
	t.Helper()
	h := fmt.Sprintf("%x", md5.Sum([]byte(t.Name()+label))) //nolint:gosec // md5 used for uniqueness, not security
	return h[:12] + "@test.com"
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

// --- POST /goals integration ---

func TestIntegration_Goal(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("POST creates goal", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
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
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, token)
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, token)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("POST different months allowed", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w1 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, token)
		w2 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":11,"goal_amount":600}`, token)
		assert.Equal(t, http.StatusCreated, w1.Code)
		assert.Equal(t, http.StatusCreated, w2.Code)
	})

	t.Run("POST same month different users allowed", func(t *testing.T) {
		tokenA := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		tokenB := registerAndLogin(t, r, uniqueEmail(t, "b"), "Senha123")
		w1 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, tokenA)
		w2 := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":500}`, tokenB)
		assert.Equal(t, http.StatusCreated, w1.Code)
		assert.Equal(t, http.StatusCreated, w2.Code)
	})

	t.Run("POST goal_amount zero returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":0}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("POST invalid month returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":13,"goal_amount":100}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("POST without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":100}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- GET /goals integration ---

func TestIntegration_Goal_List(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("GET returns empty array when no goals", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodGet, "/v1/goals", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Empty(t, resp)
	})

	t.Run("GET returns goals with status", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":1000}`, token)
		w := doReq(t, r, http.MethodGet, "/v1/goals", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, "under", resp[0]["status"])
	})

	t.Run("GET with year filter", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2025,"month":1,"goal_amount":500}`, token)
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":1,"goal_amount":800}`, token)
		w := doReq(t, r, http.MethodGet, "/v1/goals?year=2026", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, float64(2026), resp[0]["year"])
	})

	t.Run("GET with year+month filter", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":5,"goal_amount":500}`, token)
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":6,"goal_amount":600}`, token)
		w := doReq(t, r, http.MethodGet, "/v1/goals?year=2026&month=5", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, float64(5), resp[0]["month"])
	})

	t.Run("GET month without year returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodGet, "/v1/goals?month=5", "", token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("GET without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodGet, "/v1/goals", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- PUT /goals/:id integration ---

func TestIntegration_Goal_Update(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("PUT updates saved_amount", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":1000}`, token)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/goals/"+id, `{"saved_amount":500}`, token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "500.00", resp["saved_amount"])
	})

	t.Run("PUT allows zero", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":1000}`, token)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/goals/"+id, `{"saved_amount":0}`, token)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("PUT allows over goal_amount", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":1000}`, token)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/goals/"+id, `{"saved_amount":1500}`, token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "1500.00", resp["saved_amount"])
	})

	t.Run("PUT negative amount returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":1000}`, token)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/goals/"+id, `{"saved_amount":-1}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("PUT other user returns 403", func(t *testing.T) {
		tokenA := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		tokenB := registerAndLogin(t, r, uniqueEmail(t, "b"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":1000}`, tokenA)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/goals/"+id, `{"saved_amount":500}`, tokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("PUT not found returns 404", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodPut, "/v1/goals/00000000-0000-0000-0000-000000000000",
			`{"saved_amount":500}`, token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("PUT without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodPut, "/v1/goals/some-id", `{"saved_amount":500}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Handler unit tests ---

// --- GET /goals integration ---

func TestIntegration_Goal_List(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("GET returns empty array when no goals", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodGet, "/v1/goals", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Empty(t, resp)
	})

	t.Run("GET returns goals with status", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":10,"goal_amount":1000}`, token)
		w := doReq(t, r, http.MethodGet, "/v1/goals", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, "under", resp[0]["status"])
		assert.Equal(t, "1000.00", resp[0]["goal_amount"])
	})

	t.Run("GET with year filter", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2025,"month":1,"goal_amount":500}`, token)
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":1,"goal_amount":800}`, token)
		w := doReq(t, r, http.MethodGet, "/v1/goals?year=2026", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, float64(2026), resp[0]["year"])
	})

	t.Run("GET with year+month filter", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":5,"goal_amount":500}`, token)
		doReq(t, r, http.MethodPost, "/v1/goals", `{"year":2026,"month":6,"goal_amount":600}`, token)
		w := doReq(t, r, http.MethodGet, "/v1/goals?year=2026&month=5", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, float64(5), resp[0]["month"])
	})

	t.Run("GET month without year returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodGet, "/v1/goals?month=5", "", token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("GET without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodGet, "/v1/goals", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Handler unit tests ---

type mockSvc struct{ err error }

func (m *mockSvc) List(_ context.Context, _ string, _ goal.ListInput) ([]goal.GoalWithStatus, error) {
	return []goal.GoalWithStatus{}, m.err
}

func (m *mockSvc) Create(_ context.Context, _ string, _ goal.CreateInput) (goal.Goal, error) {
	return goal.Goal{}, m.err
}

func (m *mockSvc) Update(_ context.Context, _, _ string, _ goal.UpdateInput) (goal.Goal, error) {
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
