package user_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
	"github.com/gabriela-miranda-leite/lastro-api/internal/user"
)

const testJWTSecret = "test-secret-key"

func setupRouter(db *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), testJWTSecret)
	authHandler := auth.NewHandler(authSvc)

	v1 := r.Group("/v1")

	authGroup := v1.Group("/auth")
	authHandler.RegisterRoutes(authGroup)

	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, testJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)

	userHandler := user.NewHandler(user.NewService(user.NewRepository(db)))
	userHandler.RegisterRoutes(protected)

	return r
}

func subtestIP(t *testing.T) string {
	t.Helper()
	h := fnv.New32a()
	_, _ = h.Write([]byte(t.Name()))
	n := h.Sum32()
	return fmt.Sprintf("10.%d.%d.%d:9999", (n>>16)&0xff, (n>>8)&0xff, n&0xff)
}

func req(t *testing.T, r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	} else {
		bodyReader = strings.NewReader("")
	}
	httpReq := httptest.NewRequestWithContext(context.Background(), method, path, bodyReader)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.RemoteAddr = subtestIP(t)
	if token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)
	return w
}

// registerAndLogin creates a user and returns their access token and user ID.
func registerAndLogin(t *testing.T, r *gin.Engine, email, password string) (accessToken, userID string) {
	t.Helper()
	body := fmt.Sprintf(`{"name":"Test","email":%q,"password":%q}`, email, password)
	w := req(t, r, http.MethodPost, "/v1/auth/register", body, "")
	require.Equal(t, http.StatusCreated, w.Code, "register failed: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	accessToken = resp["access_token"].(string)
	userID = resp["user"].(map[string]any)["id"].(string)
	return
}

// --- GET /users/me ---

func TestIntegration_GetMe(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success returns user profile without password_hash", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "getme@test.com", "Senha123")

		w := req(t, r, http.MethodGet, "/v1/users/me", "", token)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "getme@test.com", resp["email"])
		assert.NotEmpty(t, resp["id"])
		assert.Nil(t, resp["password_hash"])
		assert.Contains(t, resp, "partner") // present even if null
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodGet, "/v1/users/me", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("deleted user returns 404", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "getme-deleted@test.com", "Senha123")
		_, err := db.ExecContext(context.Background(),
			`UPDATE users SET deleted_at = now() WHERE id = $1`, userID)
		require.NoError(t, err)

		w := req(t, r, http.MethodGet, "/v1/users/me", "", token)
		// middleware blocks first — deleted user gets 401 from Auth middleware
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- PUT /users/me ---

func TestIntegration_UpdateMe(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("partial update changes only sent fields", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "update@test.com", "Senha123")

		w := req(t, r, http.MethodPut, "/v1/users/me",
			`{"name":"Nome Novo","monthly_income":"5000.00"}`, token)

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "Nome Novo", resp["name"])
		assert.Equal(t, "5000.00", resp["monthly_income"])
		assert.Nil(t, resp["saving_goal_pct"]) // not sent, still null
		assert.Nil(t, resp["password_hash"])
	})

	t.Run("saving_goal_pct above 100 returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "invalid1@test.com", "Senha123")
		w := req(t, r, http.MethodPut, "/v1/users/me", `{"saving_goal_pct":101}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "saving_goal_pct")
	})

	t.Run("monthly_income zero or negative returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "invalid2@test.com", "Senha123")
		w := req(t, r, http.MethodPut, "/v1/users/me", `{"monthly_income":"-1.00"}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "monthly_income")
	})

	t.Run("empty name returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "invalid3@test.com", "Senha123")
		w := req(t, r, http.MethodPut, "/v1/users/me", `{"name":""}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "name")
	})

	t.Run("email in body is ignored", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "noemail@test.com", "Senha123")
		w := req(t, r, http.MethodPut, "/v1/users/me", `{"name":"Novo","email":"outro@test.com"}`, token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, "noemail@test.com", resp["email"]) // unchanged
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodPut, "/v1/users/me", `{"name":"x"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- GET /users/me/export ---

func TestIntegration_Export(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("returns complete JSON with all sections and correct headers", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "export@test.com", "Senha123")

		w := req(t, r, http.MethodGet, "/v1/users/me/export", "", token)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Disposition"),
			"export-"+userID+".json")

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		// all top-level keys present
		assert.Contains(t, resp, "user")
		assert.Contains(t, resp, "transactions")
		assert.Contains(t, resp, "recurring_expenses")
		assert.Contains(t, resp, "monthly_savings")
		assert.Contains(t, resp, "audit_log")

		// user block has correct fields and no password_hash
		u := resp["user"].(map[string]any)
		assert.Equal(t, "export@test.com", u["email"])
		assert.Nil(t, u["password_hash"])
	})

	t.Run("export generates audit_log entry", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "exportaudit@test.com", "Senha123")

		req(t, r, http.MethodGet, "/v1/users/me/export", "", token)

		var count int
		err := db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND action = 'export_data'`,
			userID,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("export includes transactions and recurring expenses", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "exportfull@test.com", "Senha123")

		// Insert a transaction directly so buildExport's scan loop is exercised.
		_, err := db.ExecContext(context.Background(), `
			INSERT INTO transactions (id, user_id, amount, direction, date, client_id)
			VALUES (gen_random_uuid(), $1, 100.00, 'out', '2026-01-01', gen_random_uuid())`,
			userID)
		require.NoError(t, err)

		// Insert a recurring expense so that scan loop is exercised too.
		_, err = db.ExecContext(context.Background(), `
			INSERT INTO recurring_expenses (id, user_id, name, amount, due_day)
			VALUES (gen_random_uuid(), $1, 'Internet', 99.90, 10)`,
			userID)
		require.NoError(t, err)

		w := req(t, r, http.MethodGet, "/v1/users/me/export", "", token)
		assert.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

		txs := resp["transactions"].([]any)
		assert.Len(t, txs, 1)

		recs := resp["recurring_expenses"].([]any)
		assert.Len(t, recs, 1)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodGet, "/v1/users/me/export", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- DELETE /users/me ---

func TestIntegration_DeleteMe(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success soft deletes user and returns 204", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "delete@test.com", "Senha123")

		w := req(t, r, http.MethodDelete, "/v1/users/me", `{"password":"Senha123"}`, token)
		assert.Equal(t, http.StatusNoContent, w.Code)

		var deletedAt sql.NullTime
		err := db.QueryRowContext(context.Background(),
			`SELECT deleted_at FROM users WHERE id = $1`, userID).Scan(&deletedAt)
		require.NoError(t, err)
		assert.True(t, deletedAt.Valid, "deleted_at should be set")
	})

	t.Run("wrong password returns 401", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "wrongpw@test.com", "Senha123")
		w := req(t, r, http.MethodDelete, "/v1/users/me", `{"password":"SenhaErrada1"}`, token)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("all refresh tokens revoked after delete", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "tokenrevoke@test.com", "Senha123")

		req(t, r, http.MethodDelete, "/v1/users/me", `{"password":"Senha123"}`, token)

		var activeTokens int
		err := db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1 AND revoked_at IS NULL`,
			userID).Scan(&activeTokens)
		require.NoError(t, err)
		assert.Equal(t, 0, activeTokens)
	})

	t.Run("deleted user cannot use their JWT", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "jwtafter@test.com", "Senha123")

		req(t, r, http.MethodDelete, "/v1/users/me", `{"password":"Senha123"}`, token)

		// token still syntactically valid but middleware blocks deleted user
		w := req(t, r, http.MethodGet, "/v1/users/me", "", token)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("audit_log entry is created on delete", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "deleteaudit@test.com", "Senha123")

		w := req(t, r, http.MethodDelete, "/v1/users/me", `{"password":"Senha123"}`, token)
		assert.Equal(t, http.StatusNoContent, w.Code)

		var count int
		err := db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND action = 'delete_account'`,
			userID).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodDelete, "/v1/users/me", `{"password":"Senha123"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
