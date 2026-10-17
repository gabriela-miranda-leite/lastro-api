package recurring_test

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
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/recurring"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

const testJWTSecret = "test-secret-key"

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

	recurringHandler := recurring.NewHandler(recurring.NewService(recurring.NewRepository(db)))
	recurringHandler.RegisterRoutes(protected)

	return r
}

func doReq(t *testing.T, r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	httpReq := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)
	return w
}

func registerAndLogin(t *testing.T, r *gin.Engine, email, password string) (token, userID string) {
	t.Helper()
	body := fmt.Sprintf(`{"name":"Test","email":%q,"password":%q}`, email, password)
	w := doReq(t, r, http.MethodPost, "/v1/auth/register", body, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["access_token"].(string), resp["user"].(map[string]any)["id"].(string)
}

// --- Integration ---

func TestIntegration_Recurring(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("POST creates recurring expense", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "rec_create@test.com", "Senha123")
		body := `{"name":"Aluguel","amount":1500,"due_day":5}`
		w := doReq(t, r, http.MethodPost, "/v1/recurring", body, token)
		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Aluguel", resp["name"])
		assert.Equal(t, "1500.00", resp["amount"])
		assert.Equal(t, float64(5), resp["due_day"])
		assert.Equal(t, userID, resp["user_id"])
		assert.Equal(t, true, resp["active"])
	})

	t.Run("POST with active=false creates inactive", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_inactive@test.com", "Senha123")
		body := `{"name":"Netflix","amount":40,"due_day":15,"active":false}`
		w := doReq(t, r, http.MethodPost, "/v1/recurring", body, token)
		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, false, resp["active"])
	})

	t.Run("POST amount zero returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_amt@test.com", "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"X","amount":0,"due_day":10}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("POST due_day out of range returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_day@test.com", "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"X","amount":10,"due_day":32}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("GET returns list of recurring expenses", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_list@test.com", "Senha123")
		doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"A","amount":100,"due_day":1}`, token)
		doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"B","amount":200,"due_day":20}`, token)

		w := doReq(t, r, http.MethodGet, "/v1/recurring", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp, 2)
	})

	t.Run("GET returns empty array when none", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_empty@test.com", "Senha123")
		w := doReq(t, r, http.MethodGet, "/v1/recurring", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Empty(t, resp)
	})

	t.Run("PUT updates name and deactivates", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_upd@test.com", "Senha123")
		wCreate := doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"Original","amount":300,"due_day":8}`, token)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/recurring/"+id, `{"name":"Atualizado","active":false}`, token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Atualizado", resp["name"])
		assert.Equal(t, false, resp["active"])
	})

	t.Run("PUT by non-owner returns 403", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "rec_updA@test.com", "Senha123")
		tokenB, _ := registerAndLogin(t, r, "rec_updB@test.com", "Senha123")
		wCreate := doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"Fixo","amount":50,"due_day":1}`, tokenA)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/recurring/"+id, `{"name":"Hack"}`, tokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("PUT unknown id returns 404", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_upd404@test.com", "Senha123")
		w := doReq(t, r, http.MethodPut, "/v1/recurring/00000000-0000-0000-0000-000000000000",
			`{"name":"X"}`, token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("DELETE removes recurring expense", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_del@test.com", "Senha123")
		wCreate := doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"Temp","amount":10,"due_day":5}`, token)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		wDel := doReq(t, r, http.MethodDelete, "/v1/recurring/"+id, "", token)
		assert.Equal(t, http.StatusNoContent, wDel.Code)

		wList := doReq(t, r, http.MethodGet, "/v1/recurring", "", token)
		var list []any
		require.NoError(t, json.Unmarshal(wList.Body.Bytes(), &list))
		assert.Empty(t, list)
	})

	t.Run("DELETE by non-owner returns 403", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "rec_delA@test.com", "Senha123")
		tokenB, _ := registerAndLogin(t, r, "rec_delB@test.com", "Senha123")
		wCreate := doReq(t, r, http.MethodPost, "/v1/recurring", `{"name":"Fixo","amount":100,"due_day":10}`, tokenA)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodDelete, "/v1/recurring/"+id, "", tokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("DELETE unknown id returns 404", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "rec_del404@test.com", "Senha123")
		w := doReq(t, r, http.MethodDelete, "/v1/recurring/00000000-0000-0000-0000-000000000000", "", token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodGet, "/v1/recurring", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Handler unit tests for 500 paths ---

type mockSvc struct{ err error }

func (m *mockSvc) List(_ context.Context, _ string) ([]recurring.Recurring, error) {
	return nil, m.err
}
func (m *mockSvc) Create(_ context.Context, _ string, _ recurring.CreateInput) (recurring.Recurring, error) {
	return recurring.Recurring{}, m.err
}
func (m *mockSvc) Update(_ context.Context, _, _ string, _ recurring.UpdateInput) (recurring.Recurring, error) {
	return recurring.Recurring{}, m.err
}
func (m *mockSvc) Delete(_ context.Context, _, _ string) error { return m.err }

func TestHandler_List_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := recurring.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/recurring", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_Create_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := recurring.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/recurring",
		strings.NewReader(`{"name":"X","amount":10,"due_day":5}`))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_Update_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := recurring.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/recurring/some-id",
		strings.NewReader(`{"name":"X"}`))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandler_Delete_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := recurring.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/recurring/some-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
