package category_test

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
	"github.com/gabriela-miranda-leite/lastro-api/internal/category"
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

	category.NewHandler(category.NewService(category.NewRepository(db))).RegisterRoutes(protected)

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

func uniqueEmail(t *testing.T, label string) string {
	t.Helper()
	h := fmt.Sprintf("%x", md5.Sum([]byte(t.Name()+label))) //nolint:gosec
	return "user_" + h[:12] + "@test.com"
}

func registerAndLogin(t *testing.T, r *gin.Engine, email, password string) string {
	t.Helper()
	body := fmt.Sprintf(`{"name":"Test","email":%q,"password":%q}`, email, password)
	doReq(t, r, http.MethodPost, "/v1/auth/register", body, "")
	w := doReq(t, r, http.MethodPost, "/v1/auth/login",
		fmt.Sprintf(`{"email":%q,"password":%q}`, email, password), "")
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["access_token"].(string)
}

// --- GET /v1/categories ---

func TestCategory_List(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("empty list", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodGet, "/v1/categories", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Empty(t, resp)
	})

	t.Run("returns own categories", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"Netflix","type":"leisure"}`, token)
		w := doReq(t, r, http.MethodGet, "/v1/categories", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp, 1)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodGet, "/v1/categories", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- POST /v1/categories ---

func TestCategory_Create(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("creates category", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"Aluguel","type":"necessity"}`, token)
		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Aluguel", resp["name"])
		assert.Equal(t, "necessity", resp["type"])
	})

	t.Run("invalid type returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"Foo","type":"invalid"}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("missing name returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodPost, "/v1/categories", `{"type":"leisure"}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"X","type":"leisure"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- PUT /v1/categories/:id ---

func TestCategory_Update(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("updates category", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"Netflix","type":"leisure"}`, token)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/categories/"+id, `{"name":"Netflix BR"}`, token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Netflix BR", resp["name"])
	})

	t.Run("invalid type returns 422", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"X","type":"leisure"}`, token)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/categories/"+id, `{"type":"invalid"}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("other user returns 403", func(t *testing.T) {
		tokenA := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		tokenB := registerAndLogin(t, r, uniqueEmail(t, "b"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"X","type":"leisure"}`, tokenA)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodPut, "/v1/categories/"+id, `{"name":"Y"}`, tokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodPut, "/v1/categories/00000000-0000-0000-0000-000000000000", `{"name":"X"}`, token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

// --- DELETE /v1/categories/:id ---

func TestCategory_Delete(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("deletes category", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"Netflix","type":"leisure"}`, token)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodDelete, "/v1/categories/"+id, "", token)
		assert.Equal(t, http.StatusNoContent, w.Code)

		wl := doReq(t, r, http.MethodGet, "/v1/categories", "", token)
		var list []any
		require.NoError(t, json.Unmarshal(wl.Body.Bytes(), &list))
		assert.Empty(t, list)
	})

	t.Run("other user returns 403", func(t *testing.T) {
		tokenA := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		tokenB := registerAndLogin(t, r, uniqueEmail(t, "b"), "Senha123")
		wc := doReq(t, r, http.MethodPost, "/v1/categories", `{"name":"X","type":"leisure"}`, tokenA)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &created))
		id := created["id"].(string)

		w := doReq(t, r, http.MethodDelete, "/v1/categories/"+id, "", tokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("not found returns 404", func(t *testing.T) {
		token := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
		w := doReq(t, r, http.MethodDelete, "/v1/categories/00000000-0000-0000-0000-000000000000", "", token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodDelete, "/v1/categories/some-id", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- unit tests ---

type mockSvc struct{ err error }

func (m *mockSvc) List(_ context.Context, _ string) ([]category.Category, error) {
	return nil, m.err
}
func (m *mockSvc) Create(_ context.Context, _ string, _ category.CreateInput) (category.Category, error) {
	return category.Category{}, m.err
}
func (m *mockSvc) Update(_ context.Context, _, _ string, _ category.UpdateInput) (category.Category, error) {
	return category.Category{}, m.err
}
func (m *mockSvc) Delete(_ context.Context, _, _ string) error { return m.err }

func TestHandler_InternalErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := category.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	t.Run("list 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/v1/categories", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("create 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/categories",
			strings.NewReader(`{"name":"X","type":"leisure"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("update 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/v1/categories/some-id",
			strings.NewReader(`{"name":"X"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("delete 500", func(t *testing.T) {
		w := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodDelete, "/v1/categories/some-id", nil)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}
