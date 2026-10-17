package couple_test

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
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
	"github.com/gabriela-miranda-leite/lastro-api/internal/couple"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
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

	coupleHandler := couple.NewHandler(couple.NewService(couple.NewRepository(db)))
	coupleHandler.RegisterRoutes(protected)

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

// --- POST /couples/invite ---

func TestIntegration_Invite(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success returns 6-char code with expiry in ~24h", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "invite@test.com", "Senha123")

		w := req(t, r, http.MethodPost, "/v1/couples/invite", "", token)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		code := resp["code"].(string)
		assert.Len(t, code, 6)
		assert.NotEmpty(t, resp["expires_at"])

		expiresAt, err := time.Parse(time.RFC3339, resp["expires_at"].(string))
		require.NoError(t, err)
		assert.WithinDuration(t, time.Now().Add(24*time.Hour), expiresAt, 5*time.Minute)
	})

	t.Run("invite code is persisted in the database", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "invitedb@test.com", "Senha123")

		w := req(t, r, http.MethodPost, "/v1/couples/invite", "", token)
		require.Equal(t, http.StatusCreated, w.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		code := resp["code"].(string)

		var storedCode string
		err := db.QueryRowContext(context.Background(),
			`SELECT code FROM couple_invites WHERE inviter_id = $1 AND used_at IS NULL ORDER BY created_at DESC LIMIT 1`,
			userID,
		).Scan(&storedCode)
		require.NoError(t, err)
		assert.Equal(t, code, storedCode)
	})

	t.Run("second invite invalidates the first", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "invitetwice@test.com", "Senha123")

		w1 := req(t, r, http.MethodPost, "/v1/couples/invite", "", token)
		require.Equal(t, http.StatusCreated, w1.Code)

		w2 := req(t, r, http.MethodPost, "/v1/couples/invite", "", token)
		require.Equal(t, http.StatusCreated, w2.Code)

		// only one active invite remains
		var activeCount int
		err := db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM couple_invites WHERE inviter_id = $1 AND used_at IS NULL AND expires_at > now()`,
			userID,
		).Scan(&activeCount)
		require.NoError(t, err)
		assert.Equal(t, 1, activeCount)
	})

	t.Run("user in couple returns 409", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "incouple@test.com", "Senha123")

		// simulate user already in a couple
		_, err := db.ExecContext(context.Background(),
			`WITH c AS (INSERT INTO couples DEFAULT VALUES RETURNING id)
			 UPDATE users SET couple_id = c.id FROM c WHERE users.id = $1`,
			userID,
		)
		require.NoError(t, err)

		w := req(t, r, http.MethodPost, "/v1/couples/invite", "", token)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodPost, "/v1/couples/invite", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- POST /couples/join ---

func TestIntegration_Join(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success links both users with same couple_id", func(t *testing.T) {
		tokenA, userA := registerAndLogin(t, r, "joina@test.com", "Senha123")
		tokenB, userB := registerAndLogin(t, r, "joinb@test.com", "Senha123")

		w := req(t, r, http.MethodPost, "/v1/couples/invite", "", tokenA)
		require.Equal(t, http.StatusCreated, w.Code)
		var inv map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inv))
		code := inv["code"].(string)

		w2 := req(t, r, http.MethodPost, "/v1/couples/join",
			fmt.Sprintf(`{"code":%q}`, code), tokenB)
		assert.Equal(t, http.StatusOK, w2.Code)

		var resp map[string]any
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &resp))
		coupleID := resp["couple_id"].(string)
		assert.NotEmpty(t, coupleID)

		// both users must share the same couple_id
		var idA, idB string
		require.NoError(t, db.QueryRowContext(context.Background(),
			`SELECT couple_id FROM users WHERE id = $1`, userA).Scan(&idA))
		require.NoError(t, db.QueryRowContext(context.Background(),
			`SELECT couple_id FROM users WHERE id = $1`, userB).Scan(&idB))
		assert.Equal(t, coupleID, idA)
		assert.Equal(t, coupleID, idB)
	})

	t.Run("invite marked as used after join", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "useda@test.com", "Senha123")
		tokenB, _ := registerAndLogin(t, r, "usedb@test.com", "Senha123")

		w := req(t, r, http.MethodPost, "/v1/couples/invite", "", tokenA)
		require.Equal(t, http.StatusCreated, w.Code)
		var inv map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inv))
		code := inv["code"].(string)

		req(t, r, http.MethodPost, "/v1/couples/join",
			fmt.Sprintf(`{"code":%q}`, code), tokenB)

		var usedAt sql.NullTime
		require.NoError(t, db.QueryRowContext(context.Background(),
			`SELECT used_at FROM couple_invites WHERE code = $1`, code).Scan(&usedAt))
		assert.True(t, usedAt.Valid)
	})

	t.Run("unknown code returns 404", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "notfound@test.com", "Senha123")
		w := req(t, r, http.MethodPost, "/v1/couples/join", `{"code":"ZZZZZZ"}`, token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("self-join returns 400", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "selfjoin@test.com", "Senha123")

		w := req(t, r, http.MethodPost, "/v1/couples/invite", "", token)
		require.Equal(t, http.StatusCreated, w.Code)
		var inv map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inv))
		code := inv["code"].(string)

		w2 := req(t, r, http.MethodPost, "/v1/couples/join",
			fmt.Sprintf(`{"code":%q}`, code), token)
		assert.Equal(t, http.StatusBadRequest, w2.Code)
	})

	t.Run("joiner already in couple returns 409", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "conflict_a@test.com", "Senha123")
		tokenB, userB := registerAndLogin(t, r, "conflict_b@test.com", "Senha123")
		tokenC, _ := registerAndLogin(t, r, "conflict_c@test.com", "Senha123")

		// A generates invite, B joins
		wInv := req(t, r, http.MethodPost, "/v1/couples/invite", "", tokenA)
		require.Equal(t, http.StatusCreated, wInv.Code)
		var inv map[string]any
		require.NoError(t, json.Unmarshal(wInv.Body.Bytes(), &inv))
		codeA := inv["code"].(string)
		req(t, r, http.MethodPost, "/v1/couples/join",
			fmt.Sprintf(`{"code":%q}`, codeA), tokenB)

		// C generates invite, B (already in couple) tries to join
		wInv2 := req(t, r, http.MethodPost, "/v1/couples/invite", "", tokenC)
		require.Equal(t, http.StatusCreated, wInv2.Code)
		var inv2 map[string]any
		require.NoError(t, json.Unmarshal(wInv2.Body.Bytes(), &inv2))
		codeC := inv2["code"].(string)

		_ = userB
		w := req(t, r, http.MethodPost, "/v1/couples/join",
			fmt.Sprintf(`{"code":%q}`, codeC), tokenB)
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodPost, "/v1/couples/join", `{"code":"ABCDEF"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
