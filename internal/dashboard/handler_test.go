package dashboard_test

import (
	"context"
	"crypto/md5" //nolint:gosec // used for email uniqueness, not security
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
	"github.com/gabriela-miranda-leite/lastro-api/internal/couple"
	"github.com/gabriela-miranda-leite/lastro-api/internal/dashboard"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
	"github.com/gabriela-miranda-leite/lastro-api/internal/transaction"
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

	couple.NewHandler(couple.NewService(couple.NewRepository(db))).RegisterRoutes(protected)
	transaction.NewHandler(transaction.NewService(transaction.NewRepository(db))).RegisterRoutes(protected)
	dashboard.NewHandler(dashboard.NewService(dashboard.NewRepository(db))).RegisterRoutes(protected)

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
	h := fmt.Sprintf("%x", md5.Sum([]byte(t.Name()+label))) //nolint:gosec // md5 used for uniqueness, not security
	return h[:12] + "@test.com"
}

// registerAndLogin returns (token, userID).
func registerAndLogin(t *testing.T, r *gin.Engine, email, password string) (token, userID string) {
	t.Helper()
	body := fmt.Sprintf(`{"name":"Test","email":%q,"password":%q}`, email, password)
	w := doReq(t, r, http.MethodPost, "/v1/auth/register", body, "")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	token = resp["access_token"].(string)
	userID = resp["user"].(map[string]any)["id"].(string)
	return
}

// insertCategory inserts a category directly in the DB and returns its ID.
func insertCategory(t *testing.T, db *sql.DB, userID, name, catType string) string {
	t.Helper()
	var id string
	err := db.QueryRowContext(context.Background(),
		`INSERT INTO categories (user_id, name, type) VALUES ($1, $2, $3) RETURNING id`,
		userID, name, catType,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// postTx posts a transaction and returns its ID.
func postTx(t *testing.T, r *gin.Engine, token, clientID string, amount float64, direction, date, categoryID string, isShared bool) string {
	t.Helper()
	var catPart string
	if categoryID != "" {
		catPart = fmt.Sprintf(`,"category_id":%q`, categoryID)
	}
	body := fmt.Sprintf(
		`{"client_id":%q,"amount":%v,"direction":%q,"date":%q,"is_shared":%v%s}`,
		clientID, amount, direction, date, isShared, catPart,
	)
	w := doReq(t, r, http.MethodPost, "/v1/transactions", body, token)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["id"].(string)
}

// formCouple has user A invite and user B join, returning both tokens and their IDs.
func formCouple(t *testing.T, r *gin.Engine, emailA, emailB string) (tokenA, idA, tokenB, idB string) {
	t.Helper()
	tokenA, idA = registerAndLogin(t, r, emailA, "Senha123")
	tokenB, idB = registerAndLogin(t, r, emailB, "Senha123")

	w := doReq(t, r, http.MethodPost, "/v1/couples/invite", "", tokenA)
	require.Equal(t, http.StatusCreated, w.Code)
	var inv map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inv))
	code := inv["code"].(string)

	w = doReq(t, r, http.MethodPost, "/v1/couples/join", fmt.Sprintf(`{"code":%q}`, code), tokenB)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	return
}

// getDashboard returns the parsed dashboard response for year/month.
func getDashboard(t *testing.T, r *gin.Engine, token string, year, month int) map[string]any {
	t.Helper()
	w := doReq(t, r, http.MethodGet, fmt.Sprintf("/v1/dashboard/%d/%d", year, month), "", token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// byType extracts expenses.by_type[catType] from a dashboard response.
func byType(t *testing.T, resp map[string]any, catType string) map[string]any {
	t.Helper()
	expenses := resp["expenses"].(map[string]any)
	bt := expenses["by_type"].(map[string]any)
	return bt[catType].(map[string]any)
}

// --- Tests ---

func TestIntegration_Dashboard_EmptyMonth(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, _ := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	resp := getDashboard(t, r, token, 2026, 10)

	assert.Equal(t, float64(2026), resp["year"])
	assert.Equal(t, float64(10), resp["month"])
	assert.Equal(t, "0.00", resp["income"])
	assert.Nil(t, resp["couple"])
	assert.Nil(t, resp["savings"])

	expenses := resp["expenses"].(map[string]any)
	assert.Equal(t, "0.00", expenses["total"])

	for _, ct := range []string{"necessity", "leisure", "investment", "saving"} {
		bt := expenses["by_type"].(map[string]any)[ct].(map[string]any)
		assert.Equal(t, "0.00", bt["amount"], "type=%s amount", ct)
		assert.Equal(t, "0.00", bt["pct"], "type=%s pct", ct)
		assert.Equal(t, "under", bt["status"], "type=%s status", ct)
	}
}

func TestIntegration_Dashboard_WithTransactions(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	necCat := insertCategory(t, db, userID, "Alimentação", "necessity")
	leiCat := insertCategory(t, db, userID, "Cinema", "leisure")

	// income: 5000
	postTx(t, r, token, "c0000001-0000-0000-0000-000000000001", 5000, "in", "2026-10-05", "", false)
	// necessity: 1000 (20% of 5000, target 50% → under)
	postTx(t, r, token, "c0000001-0000-0000-0000-000000000002", 1000, "out", "2026-10-10", necCat, false)
	// leisure: 500 (10% of 5000, target 30% → under)
	postTx(t, r, token, "c0000001-0000-0000-0000-000000000003", 500, "out", "2026-10-11", leiCat, false)

	resp := getDashboard(t, r, token, 2026, 10)

	assert.Equal(t, "5000.00", resp["income"])

	expenses := resp["expenses"].(map[string]any)
	assert.Equal(t, "1500.00", expenses["total"])

	nec := byType(t, resp, "necessity")
	assert.Equal(t, "1000.00", nec["amount"])
	assert.Equal(t, "20.00", nec["pct"])
	assert.Equal(t, "50", nec["target_pct"])
	assert.Equal(t, "under", nec["status"])

	lei := byType(t, resp, "leisure")
	assert.Equal(t, "500.00", lei["amount"])
	assert.Equal(t, "10.00", lei["pct"])
	assert.Equal(t, "30", lei["target_pct"])
	assert.Equal(t, "under", lei["status"])

	// no transactions in investment/saving → zero
	inv := byType(t, resp, "investment")
	assert.Equal(t, "0.00", inv["amount"])
	assert.Equal(t, "0.00", inv["pct"])
}

func TestIntegration_Dashboard_StatusMetAndOver(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	necCat := insertCategory(t, db, userID, "Moradia", "necessity")
	leiCat := insertCategory(t, db, userID, "Viagem", "leisure")

	// income: 1000
	postTx(t, r, token, "c0000002-0000-0000-0000-000000000001", 1000, "in", "2026-10-01", "", false)
	// necessity: exactly 500 (50% of 1000 → met)
	postTx(t, r, token, "c0000002-0000-0000-0000-000000000002", 500, "out", "2026-10-01", necCat, false)
	// leisure: 400 (40% of 1000, target 30% → over)
	postTx(t, r, token, "c0000002-0000-0000-0000-000000000003", 400, "out", "2026-10-01", leiCat, false)

	resp := getDashboard(t, r, token, 2026, 10)

	assert.Equal(t, "met", byType(t, resp, "necessity")["status"])
	assert.Equal(t, "over", byType(t, resp, "leisure")["status"])
}

func TestIntegration_Dashboard_IncomeZero(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	necCat := insertCategory(t, db, userID, "Mercado", "necessity")
	postTx(t, r, token, "c0000003-0000-0000-0000-000000000001", 200, "out", "2026-10-01", necCat, false)

	resp := getDashboard(t, r, token, 2026, 10)

	assert.Equal(t, "0.00", resp["income"])
	nec := byType(t, resp, "necessity")
	assert.Equal(t, "200.00", nec["amount"])
	assert.Equal(t, "0.00", nec["pct"]) // no division by zero
}

func TestIntegration_Dashboard_NoCouple(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, _ := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	resp := getDashboard(t, r, token, 2026, 10)
	assert.Nil(t, resp["couple"])
}

func TestIntegration_Dashboard_WithCouple(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	tokenA, idA, tokenB, idB := formCouple(t, r, uniqueEmail(t, "a"), uniqueEmail(t, "b"))

	necA := insertCategory(t, db, idA, "Aluguel", "necessity")
	necB := insertCategory(t, db, idB, "Conta", "necessity")

	// A earns 4000, spends 1000 necessity
	postTx(t, r, tokenA, "c0000004-0000-0000-0000-000000000001", 4000, "in", "2026-10-01", "", false)
	postTx(t, r, tokenA, "c0000004-0000-0000-0000-000000000002", 1000, "out", "2026-10-01", necA, false)

	// B earns 2000, spends 500 necessity
	postTx(t, r, tokenB, "c0000004-0000-0000-0000-000000000003", 2000, "in", "2026-10-01", "", false)
	postTx(t, r, tokenB, "c0000004-0000-0000-0000-000000000004", 500, "out", "2026-10-01", necB, false)

	resp := getDashboard(t, r, tokenA, 2026, 10)

	// individual A
	assert.Equal(t, "4000.00", resp["income"])
	assert.Equal(t, "1000.00", resp["expenses"].(map[string]any)["total"])

	// couple consolidated: income = 6000, expenses = 1500
	coupleResp := resp["couple"].(map[string]any)
	assert.Equal(t, "6000.00", coupleResp["income"])
	coupleExpenses := coupleResp["expenses"].(map[string]any)
	assert.Equal(t, "1500.00", coupleExpenses["total"])

	// couple necessity pct = 1500/6000*100 = 25.00 → under (target 50%)
	coupleNec := coupleExpenses["by_type"].(map[string]any)["necessity"].(map[string]any)
	assert.Equal(t, "1500.00", coupleNec["amount"])
	assert.Equal(t, "25.00", coupleNec["pct"])
	assert.Equal(t, "under", coupleNec["status"])
}

func TestIntegration_Dashboard_IsSharedNotDuplicated(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	tokenA, idA, tokenB, _ := formCouple(t, r, uniqueEmail(t, "a"), uniqueEmail(t, "b"))

	necA := insertCategory(t, db, idA, "Supermercado", "necessity")

	postTx(t, r, tokenA, "c0000005-0000-0000-0000-000000000001", 3000, "in", "2026-10-01", "", false)
	postTx(t, r, tokenB, "c0000005-0000-0000-0000-000000000002", 3000, "in", "2026-10-01", "", false)
	// A posts a shared expense of 600
	postTx(t, r, tokenA, "c0000005-0000-0000-0000-000000000003", 600, "out", "2026-10-01", necA, true)

	resp := getDashboard(t, r, tokenA, 2026, 10)

	// individual A sees 600
	assert.Equal(t, "600.00", resp["expenses"].(map[string]any)["total"])

	// couple sees 600 only once (not 1200)
	coupleExpenses := resp["couple"].(map[string]any)["expenses"].(map[string]any)
	assert.Equal(t, "600.00", coupleExpenses["total"])
}

func TestIntegration_Dashboard_InvalidParams(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, _ := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	t.Run("invalid year returns 422", func(t *testing.T) {
		w := doReq(t, r, http.MethodGet, "/v1/dashboard/abc/10", "", token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("invalid month returns 422", func(t *testing.T) {
		w := doReq(t, r, http.MethodGet, "/v1/dashboard/2026/13", "", token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("no token returns 401", func(t *testing.T) {
		w := doReq(t, r, http.MethodGet, "/v1/dashboard/2026/10", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}
