package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

const jwtSecret = "test-secret"

func makeToken(t *testing.T, sub string, exp time.Time, key string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": sub,
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	})
	s, err := tok.SignedString([]byte(key))
	require.NoError(t, err)
	return s
}

func authRouter(t *testing.T) (engine *gin.Engine, userID string) {
	t.Helper()
	db := testhelpers.SetupTestDB(t)

	// Register a real user so the middleware can check it.
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (name, email, password_hash)
		 VALUES ('Test', 'mw@test.com', 'hash') RETURNING id`,
	).Scan(&userID)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	engine = gin.New()
	engine.Use(middleware.Auth(db, jwtSecret))
	engine.GET("/protected", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
	})
	return engine, userID
}

func getProtected(r *gin.Engine, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/protected", http.NoBody)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAuth_MissingToken(t *testing.T) {
	r, _ := authRouter(t)
	w := getProtected(r, "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_InvalidSignature(t *testing.T) {
	r, userID := authRouter(t)
	tok := makeToken(t, userID, time.Now().Add(time.Hour), "wrong-secret")
	w := getProtected(r, tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ExpiredToken(t *testing.T) {
	r, userID := authRouter(t)
	tok := makeToken(t, userID, time.Now().Add(-time.Hour), jwtSecret)
	w := getProtected(r, tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_UserNotFound(t *testing.T) {
	r, _ := authRouter(t)
	tok := makeToken(t, "00000000-0000-0000-0000-000000000000", time.Now().Add(time.Hour), jwtSecret)
	w := getProtected(r, tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAuth_ValidToken(t *testing.T) {
	r, userID := authRouter(t)
	tok := makeToken(t, userID, time.Now().Add(time.Hour), jwtSecret)
	w := getProtected(r, tok)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), userID)
}

func TestAuth_DeletedUser(t *testing.T) {
	db := testhelpers.SetupTestDB(t)

	var userID string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO users (name, email, password_hash, deleted_at)
		 VALUES ('Del', 'deleted@test.com', 'hash', now()) RETURNING id`,
	).Scan(&userID)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.Auth(db, jwtSecret))
	r.GET("/protected", func(c *gin.Context) { c.Status(http.StatusOK) })

	tok := makeToken(t, userID, time.Now().Add(time.Hour), jwtSecret)
	w := getProtected(r, tok)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- RateLimit ---

func TestRateLimit_Allows(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimit("100-M"))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimit_Blocks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.RateLimit("1-M"))
	r.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func() int {
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ping", http.NoBody)
		req.RemoteAddr = "10.0.0.2:9999"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	assert.Equal(t, http.StatusOK, do())
	assert.Equal(t, http.StatusTooManyRequests, do())
}

func TestRateLimit_InvalidFormat(t *testing.T) {
	assert.Panics(t, func() {
		middleware.RateLimit("bad-format")
	})
}
