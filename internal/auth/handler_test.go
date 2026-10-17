package auth_test

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
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

const testJWTSecret = "test-secret-key"

func setupRouter(db *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// trust all proxies in tests so X-Forwarded-For is used as ClientIP
	_ = r.SetTrustedProxies(nil)

	svc := auth.NewService(auth.NewRepository(db), testJWTSecret)
	h := auth.NewHandler(svc)

	v1 := r.Group("/v1")
	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimit("5-M"))
	h.RegisterRoutes(authGroup)

	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, testJWTSecret))
	h.RegisterProtectedRoutes(protected)

	return r
}

// subtestIP returns a stable unique IP for a subtest so requests from different
// subtests don't share rate-limit quota in the same router instance.
func subtestIP(t *testing.T) string {
	t.Helper()
	h := fnv.New32a()
	_, _ = h.Write([]byte(t.Name()))
	n := h.Sum32()
	return fmt.Sprintf("10.%d.%d.%d:9999", (n>>16)&0xff, (n>>8)&0xff, n&0xff)
}

func post(t *testing.T, r *gin.Engine, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = subtestIP(t)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func registerUser(t *testing.T, r *gin.Engine, email, password string) map[string]any {
	t.Helper()
	body := fmt.Sprintf(`{"name":"Test","email":%q,"password":%q}`, email, password)
	w := post(t, r, "/v1/auth/register", body, "")
	require.Equal(t, http.StatusCreated, w.Code, "register failed: %s", w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// --- Register ---

func TestIntegration_Register(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success returns 201 with user and tokens", func(t *testing.T) {
		w := post(t, r, "/v1/auth/register",
			`{"name":"Ana","email":"ana@test.com","password":"Senha123"}`, "")

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp["access_token"])
		assert.NotEmpty(t, resp["refresh_token"])
		user := resp["user"].(map[string]any)
		assert.Equal(t, "ana@test.com", user["email"])
		assert.NotEmpty(t, user["id"])
		assert.Empty(t, user["password_hash"]) // nunca expor hash
	})

	t.Run("duplicate email returns 409", func(t *testing.T) {
		registerUser(t, r, "dup@test.com", "Senha123")
		w := post(t, r, "/v1/auth/register",
			`{"name":"Ana","email":"dup@test.com","password":"Senha123"}`, "")
		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("weak password returns 422", func(t *testing.T) {
		tests := []struct{ name, password string }{
			{"too short", "Ab1"},
			{"no uppercase", "senha123"},
			{"no digit", "SenhaSemNumero"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				body := fmt.Sprintf(`{"name":"x","email":"x@test.com","password":%q}`, tt.password)
				w := post(t, r, "/v1/auth/register", body, "")
				assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
			})
		}
	})

	t.Run("missing required field returns 422", func(t *testing.T) {
		w := post(t, r, "/v1/auth/register", `{"name":"Ana","password":"Senha123"}`, "")
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})
}

// --- Login ---

func TestIntegration_Login(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	registerUser(t, r, "login@test.com", "Senha123")

	t.Run("success returns 200 with access and refresh tokens", func(t *testing.T) {
		w := post(t, r, "/v1/auth/login",
			`{"email":"login@test.com","password":"Senha123"}`, "")

		assert.Equal(t, http.StatusOK, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.NotEmpty(t, resp["access_token"])
		assert.NotEmpty(t, resp["refresh_token"])

		// JWT deve ser decodificável e ter claims corretos
		tokenStr := resp["access_token"].(string)
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			return []byte(testJWTSecret), nil
		}, jwt.WithValidMethods([]string{"HS256"}))
		require.NoError(t, err)
		claims := token.Claims.(jwt.MapClaims)
		assert.NotEmpty(t, claims["sub"])
		assert.NotEmpty(t, claims["exp"])
	})

	t.Run("nonexistent email returns 401 with generic message", func(t *testing.T) {
		w := post(t, r, "/v1/auth/login",
			`{"email":"nao@existe.com","password":"Senha123"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid credentials")
	})

	t.Run("wrong password returns 401 with same generic message", func(t *testing.T) {
		w := post(t, r, "/v1/auth/login",
			`{"email":"login@test.com","password":"SenhaErrada1"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.Contains(t, w.Body.String(), "invalid credentials")
	})

	t.Run("deleted account returns 401", func(t *testing.T) {
		registerUser(t, r, "deleted@test.com", "Senha123")
		_, err := db.ExecContext(context.Background(),
			`UPDATE users SET deleted_at = now() WHERE email = 'deleted@test.com'`)
		require.NoError(t, err)

		w := post(t, r, "/v1/auth/login",
			`{"email":"deleted@test.com","password":"Senha123"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("rate limit blocks after 5 requests", func(t *testing.T) {
		// use a dedicated router instance so the in-memory limiter starts fresh
		rl := setupRouter(db)
		makeReq := func() int {
			req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/auth/login",
				strings.NewReader(`{"email":"x@x.com","password":"SenhaErrada1"}`))
			req.Header.Set("Content-Type", "application/json")
			req.RemoteAddr = "10.0.1.99:1234"
			w := httptest.NewRecorder()
			rl.ServeHTTP(w, req)
			return w.Code
		}

		for i := 0; i < 5; i++ {
			code := makeReq()
			assert.Equal(t, http.StatusUnauthorized, code, "request %d should be 401", i+1)
		}
		assert.Equal(t, http.StatusTooManyRequests, makeReq(), "6th request should be 429")
	})
}

// --- Refresh ---

func TestIntegration_Refresh(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success returns new token pair and invalidates old refresh token", func(t *testing.T) {
		resp := registerUser(t, r, "refresh@test.com", "Senha123")
		oldRefresh := resp["refresh_token"].(string)

		w := post(t, r, "/v1/auth/refresh",
			fmt.Sprintf(`{"refresh_token":%q}`, oldRefresh), "")
		require.Equal(t, http.StatusOK, w.Code)

		var newResp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &newResp))
		assert.NotEmpty(t, newResp["access_token"])
		newRefresh := newResp["refresh_token"].(string)
		assert.NotEqual(t, oldRefresh, newRefresh)

		// token anterior não deve funcionar mais
		w2 := post(t, r, "/v1/auth/refresh",
			fmt.Sprintf(`{"refresh_token":%q}`, oldRefresh), "")
		assert.Equal(t, http.StatusUnauthorized, w2.Code)
	})

	t.Run("reuse of revoked token returns 401", func(t *testing.T) {
		resp := registerUser(t, r, "reuse@test.com", "Senha123")
		refresh := resp["refresh_token"].(string)

		// usar uma vez (válido)
		post(t, r, "/v1/auth/refresh", fmt.Sprintf(`{"refresh_token":%q}`, refresh), "")

		// reutilizar token revogado → 401
		w := post(t, r, "/v1/auth/refresh",
			fmt.Sprintf(`{"refresh_token":%q}`, refresh), "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("nonexistent token returns 401", func(t *testing.T) {
		w := post(t, r, "/v1/auth/refresh", `{"refresh_token":"token-que-nao-existe"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		resp := registerUser(t, r, "expired@test.com", "Senha123")
		refresh := resp["refresh_token"].(string)

		// forçar expiração no banco
		_, err := db.ExecContext(context.Background(),
			`UPDATE refresh_tokens SET expires_at = now() - interval '1 second'
			 WHERE user_id = (SELECT id FROM users WHERE email = 'expired@test.com')`)
		require.NoError(t, err)

		w := post(t, r, "/v1/auth/refresh",
			fmt.Sprintf(`{"refresh_token":%q}`, refresh), "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Logout ---

func TestIntegration_Logout(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success returns 204 and token is invalidated", func(t *testing.T) {
		resp := registerUser(t, r, "logout@test.com", "Senha123")
		access := resp["access_token"].(string)
		refresh := resp["refresh_token"].(string)

		w := post(t, r, "/v1/auth/logout",
			fmt.Sprintf(`{"refresh_token":%q}`, refresh), access)
		assert.Equal(t, http.StatusNoContent, w.Code)

		// token não deve funcionar em /refresh
		w2 := post(t, r, "/v1/auth/refresh",
			fmt.Sprintf(`{"refresh_token":%q}`, refresh), "")
		assert.Equal(t, http.StatusUnauthorized, w2.Code)
	})

	t.Run("already revoked token returns 204 (idempotent)", func(t *testing.T) {
		resp := registerUser(t, r, "logout2@test.com", "Senha123")
		access := resp["access_token"].(string)
		refresh := resp["refresh_token"].(string)

		post(t, r, "/v1/auth/logout", fmt.Sprintf(`{"refresh_token":%q}`, refresh), access)
		w := post(t, r, "/v1/auth/logout",
			fmt.Sprintf(`{"refresh_token":%q}`, refresh), access)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	t.Run("without JWT returns 401", func(t *testing.T) {
		w := post(t, r, "/v1/auth/logout", `{"refresh_token":"qualquer"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- Middleware JWT ---

func TestIntegration_JWTMiddleware(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("missing token returns 401", func(t *testing.T) {
		w := post(t, r, "/v1/auth/logout", `{"refresh_token":"x"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("expired token returns 401", func(t *testing.T) {
		claims := jwt.MapClaims{
			"sub": "user-1",
			"exp": time.Now().Add(-time.Hour).Unix(),
			"iat": time.Now().Add(-2 * time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, _ := token.SignedString([]byte(testJWTSecret))

		w := post(t, r, "/v1/auth/logout", `{"refresh_token":"x"}`, signed)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid signature returns 401", func(t *testing.T) {
		claims := jwt.MapClaims{
			"sub": "user-1",
			"exp": time.Now().Add(time.Hour).Unix(),
			"iat": time.Now().Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, _ := token.SignedString([]byte("wrong-secret"))

		w := post(t, r, "/v1/auth/logout", `{"refresh_token":"x"}`, signed)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("token of deleted user returns 401", func(t *testing.T) {
		resp := registerUser(t, r, "todelete@test.com", "Senha123")
		access := resp["access_token"].(string)

		_, err := db.ExecContext(context.Background(),
			`UPDATE users SET deleted_at = now() WHERE email = 'todelete@test.com'`)
		require.NoError(t, err)

		w := post(t, r, "/v1/auth/logout", `{"refresh_token":"x"}`, access)
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("valid token passes and user_id is in context", func(t *testing.T) {
		resp := registerUser(t, r, "valid@test.com", "Senha123")
		access := resp["access_token"].(string)
		refresh := resp["refresh_token"].(string)

		// logout é a rota protegida mais simples para testar o middleware
		w := post(t, r, "/v1/auth/logout",
			fmt.Sprintf(`{"refresh_token":%q}`, refresh), access)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
