package transaction_test

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
	"github.com/gabriela-miranda-leite/lastro-api/internal/couple"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
	"github.com/gabriela-miranda-leite/lastro-api/internal/transaction"
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

	txHandler := transaction.NewHandler(transaction.NewService(transaction.NewRepository(db)))
	txHandler.RegisterRoutes(protected)

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
	httpReq := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.RemoteAddr = subtestIP(t)
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
	w := req(t, r, http.MethodPost, "/v1/auth/register", body, "")
	require.Equal(t, http.StatusCreated, w.Code, "register: %s", w.Body)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp["access_token"].(string), resp["user"].(map[string]any)["id"].(string)
}

// createCouple links two users and returns their tokens.
func createCouple(t *testing.T, r *gin.Engine, emailA, emailB string) (tokenA, tokenB string) {
	t.Helper()
	tokenA, _ = registerAndLogin(t, r, emailA, "Senha123")
	tokenB, _ = registerAndLogin(t, r, emailB, "Senha123")

	wInv := req(t, r, http.MethodPost, "/v1/couples/invite", "", tokenA)
	require.Equal(t, http.StatusCreated, wInv.Code)
	var inv map[string]any
	require.NoError(t, json.Unmarshal(wInv.Body.Bytes(), &inv))

	wJoin := req(t, r, http.MethodPost, "/v1/couples/join",
		fmt.Sprintf(`{"code":%q}`, inv["code"]), tokenB)
	require.Equal(t, http.StatusOK, wJoin.Code)
	return
}

// --- POST /transactions ---

func TestIntegration_CreateTransaction(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("success returns 201 with created transaction", func(t *testing.T) {
		token, userID := registerAndLogin(t, r, "tx_ok@test.com", "Senha123")

		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440001","amount":"150.50","direction":"EXPENSE","date":"2026-10-17","is_shared":false}`
		w := req(t, r, http.MethodPost, "/v1/transactions", body, token)

		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, userID, resp["user_id"])
		assert.Equal(t, "150.50", resp["amount"])
		assert.Equal(t, "EXPENSE", resp["direction"])
		assert.Equal(t, "2026-10-17", resp["date"])
		assert.NotEmpty(t, resp["id"])
	})

	t.Run("duplicate client_id returns 200 with existing transaction", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "tx_idem@test.com", "Senha123")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440002","amount":"50.00","direction":"INCOME","date":"2026-10-17"}`

		w1 := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		require.Equal(t, http.StatusCreated, w1.Code)
		var first map[string]any
		require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &first))

		w2 := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		assert.Equal(t, http.StatusOK, w2.Code)
		var second map[string]any
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &second))
		assert.Equal(t, first["id"], second["id"], "idempotent: same transaction returned")
	})

	t.Run("amount zero returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "tx_amt@test.com", "Senha123")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440003","amount":"0","direction":"EXPENSE","date":"2026-10-17"}`
		w := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "amount")
	})

	t.Run("invalid direction returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "tx_dir@test.com", "Senha123")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440004","amount":"10","direction":"sideways","date":"2026-10-17"}`
		w := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "direction")
	})

	t.Run("invalid date format returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "tx_date@test.com", "Senha123")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440005","amount":"10","direction":"EXPENSE","date":"17/10/2026"}`
		w := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
		assert.Contains(t, w.Body.String(), "date")
	})

	t.Run("is_shared without couple returns 400", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "tx_shared@test.com", "Senha123")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440006","amount":"10","direction":"EXPENSE","date":"2026-10-17","is_shared":true}`
		w := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("is_shared with couple returns 201", func(t *testing.T) {
		tokenA, _ := createCouple(t, r, "tx_couple_a@test.com", "tx_couple_b@test.com")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440007","amount":"200","direction":"EXPENSE","date":"2026-10-17","is_shared":true}`
		w := req(t, r, http.MethodPost, "/v1/transactions", body, tokenA)
		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, true, resp["is_shared"])
	})

	t.Run("category from another user returns 403", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "tx_catA@test.com", "Senha123")
		tokenB, _ := registerAndLogin(t, r, "tx_catB@test.com", "Senha123")

		// B creates a category
		var catID string
		require.NoError(t, db.QueryRowContext(context.Background(),
			`INSERT INTO categories (user_id, name, type) VALUES ((SELECT id FROM users WHERE email='tx_catB@test.com'), 'Lazer', 'WANTS') RETURNING id`,
		).Scan(&catID))

		body := fmt.Sprintf(`{"client_id":"550e8400-e29b-41d4-a716-446655440008","amount":"10","direction":"EXPENSE","date":"2026-10-17","category_id":%q}`, catID)
		_ = tokenB
		w := req(t, r, http.MethodPost, "/v1/transactions", body, tokenA)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("own category is accepted", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "tx_owncat@test.com", "Senha123")

		var catID string
		require.NoError(t, db.QueryRowContext(context.Background(),
			`INSERT INTO categories (user_id, name, type) VALUES ((SELECT id FROM users WHERE email='tx_owncat@test.com'), 'Alimentação', 'NEEDS') RETURNING id`,
		).Scan(&catID))

		body := fmt.Sprintf(`{"client_id":"550e8400-e29b-41d4-a716-446655440009","amount":"30","direction":"EXPENSE","date":"2026-10-17","category_id":%q}`, catID)
		w := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		assert.Equal(t, http.StatusCreated, w.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Equal(t, catID, resp["category_id"])
	})

	t.Run("without token returns 401", func(t *testing.T) {
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655440010","amount":"10","direction":"EXPENSE","date":"2026-10-17"}`
		w := req(t, r, http.MethodPost, "/v1/transactions", body, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

// --- handler unit tests for 500 paths ---

func TestHandler_Create_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := transaction.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/transactions",
		strings.NewReader(`{"client_id":"550e8400-e29b-41d4-a716-446655440099","amount":"10","direction":"EXPENSE","date":"2026-10-17"}`))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

// mockSvc satisfies transactionService for handler unit tests.
type mockSvc struct{ err error }

func (m *mockSvc) Create(_ context.Context, _ string, _ *transaction.CreateInput) (transaction.Transaction, bool, error) {
	return transaction.Transaction{}, false, m.err
}

func (m *mockSvc) List(_ context.Context, _ string, _ transaction.ListInput) ([]transaction.Transaction, error) {
	return nil, m.err
}

func (m *mockSvc) Update(_ context.Context, _, _ string, _ transaction.UpdateInput) (transaction.Transaction, error) {
	return transaction.Transaction{}, m.err
}

func (m *mockSvc) Delete(_ context.Context, _, _ string) error {
	return m.err
}

// --- GET /transactions ---

func TestIntegration_ListTransactions(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("returns empty array when no transactions", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "list_empty@test.com", "Senha123")
		w := req(t, r, http.MethodGet, "/v1/transactions?year=2026&month=10", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Empty(t, resp)
	})

	t.Run("returns own transactions for the month", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "list_own@test.com", "Senha123")

		req1 := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655441001","amount":"50","direction":"EXPENSE","date":"2026-10-05"}`,
			token)
		require.Equal(t, http.StatusCreated, req1.Code)

		// different month — should not appear
		req2 := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655441002","amount":"20","direction":"INCOME","date":"2026-09-01"}`,
			token)
		require.Equal(t, http.StatusCreated, req2.Code)

		w := req(t, r, http.MethodGet, "/v1/transactions?year=2026&month=10", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp, 1)
	})

	t.Run("filter by direction", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "list_dir@test.com", "Senha123")

		req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655441003","amount":"10","direction":"INCOME","date":"2026-10-01"}`,
			token)
		req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655441004","amount":"20","direction":"EXPENSE","date":"2026-10-02"}`,
			token)

		w := req(t, r, http.MethodGet, "/v1/transactions?year=2026&month=10&direction=INCOME", "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp, 1)
		assert.Equal(t, "INCOME", resp[0]["direction"])
	})

	t.Run("filter by category_id", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "list_cat@test.com", "Senha123")

		var catID string
		require.NoError(t, db.QueryRowContext(context.Background(),
			`INSERT INTO categories (user_id, name, type) VALUES ((SELECT id FROM users WHERE email='list_cat@test.com'), 'Mercado', 'NEEDS') RETURNING id`,
		).Scan(&catID))

		req(t, r, http.MethodPost, "/v1/transactions",
			fmt.Sprintf(`{"client_id":"550e8400-e29b-41d4-a716-446655441010","amount":"50","direction":"EXPENSE","date":"2026-10-01","category_id":%q}`, catID),
			token)
		req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655441011","amount":"30","direction":"EXPENSE","date":"2026-10-02"}`,
			token)

		w := req(t, r, http.MethodGet,
			fmt.Sprintf("/v1/transactions?year=2026&month=10&category_id=%s", catID), "", token)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp, 1)
		assert.Equal(t, catID, resp[0]["category_id"])
	})

	t.Run("access partner transactions in couple", func(t *testing.T) {
		tokenA, userIDA := registerAndLogin(t, r, "list_coupleA@test.com", "Senha123")
		tokenB, userIDB := registerAndLogin(t, r, "list_coupleB@test.com", "Senha123")
		_ = userIDA

		wInv := req(t, r, http.MethodPost, "/v1/couples/invite", "", tokenA)
		require.Equal(t, http.StatusCreated, wInv.Code)
		var inv map[string]any
		require.NoError(t, json.Unmarshal(wInv.Body.Bytes(), &inv))
		wJoin := req(t, r, http.MethodPost, "/v1/couples/join",
			fmt.Sprintf(`{"code":%q}`, inv["code"]), tokenB)
		require.Equal(t, http.StatusOK, wJoin.Code)

		// B creates a transaction
		req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655441005","amount":"75","direction":"EXPENSE","date":"2026-10-10"}`,
			tokenB)

		// A queries B's transactions
		w := req(t, r, http.MethodGet,
			fmt.Sprintf("/v1/transactions?year=2026&month=10&user_id=%s", userIDB), "", tokenA)
		assert.Equal(t, http.StatusOK, w.Code)
		var resp []map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		assert.Len(t, resp, 1)
		assert.Equal(t, userIDB, resp[0]["user_id"])
	})

	t.Run("access transactions of unrelated user returns 403", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "list_403A@test.com", "Senha123")
		_, userIDB := registerAndLogin(t, r, "list_403B@test.com", "Senha123")

		w := req(t, r, http.MethodGet,
			fmt.Sprintf("/v1/transactions?year=2026&month=10&user_id=%s", userIDB), "", tokenA)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("missing year returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "list_noyear@test.com", "Senha123")
		w := req(t, r, http.MethodGet, "/v1/transactions?month=10", "", token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("missing month returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "list_nomonth@test.com", "Senha123")
		w := req(t, r, http.MethodGet, "/v1/transactions?year=2026", "", token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodGet, "/v1/transactions?year=2026&month=10", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandler_List_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := transaction.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(),
		http.MethodGet, "/v1/transactions?year=2026&month=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

// --- PUT /transactions/:id ---

func TestIntegration_UpdateTransaction(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("owner can update amount and date", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "upd_ok@test.com", "Senha123")

		wCreate := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655442001","amount":"100","direction":"EXPENSE","date":"2026-10-01"}`,
			token)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		wUpd := req(t, r, http.MethodPut, "/v1/transactions/"+id,
			`{"amount":"250.75","date":"2026-10-15"}`, token)
		assert.Equal(t, http.StatusOK, wUpd.Code)
		var resp map[string]any
		require.NoError(t, json.Unmarshal(wUpd.Body.Bytes(), &resp))
		assert.Equal(t, "250.75", resp["amount"])
		assert.Equal(t, "2026-10-15", resp["date"])
	})

	t.Run("non-owner gets 403", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "upd_ownerA@test.com", "Senha123")
		tokenB, _ := registerAndLogin(t, r, "upd_ownerB@test.com", "Senha123")

		wCreate := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655442002","amount":"50","direction":"INCOME","date":"2026-10-01"}`,
			tokenA)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		w := req(t, r, http.MethodPut, "/v1/transactions/"+id, `{"amount":"99"}`, tokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("unknown id returns 404", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "upd_404@test.com", "Senha123")
		w := req(t, r, http.MethodPut, "/v1/transactions/00000000-0000-0000-0000-000000000000",
			`{"amount":"10"}`, token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("invalid amount returns 422", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "upd_amt@test.com", "Senha123")

		wCreate := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655442003","amount":"30","direction":"EXPENSE","date":"2026-10-01"}`,
			token)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		w := req(t, r, http.MethodPut, "/v1/transactions/"+id, `{"amount":-1}`, token)
		assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodPut, "/v1/transactions/some-id", `{"amount":"10"}`, "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandler_Update_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := transaction.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(),
		http.MethodPut, "/v1/transactions/some-id",
		strings.NewReader(`{"amount":"10"}`))
	httpReq.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

// --- DELETE /transactions/:id ---

func TestIntegration_DeleteTransaction(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("owner deletes returns 204 and transaction disappears from list", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "del_ok@test.com", "Senha123")

		wCreate := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655443001","amount":"80","direction":"EXPENSE","date":"2026-10-05"}`,
			token)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		wDel := req(t, r, http.MethodDelete, "/v1/transactions/"+id, "", token)
		assert.Equal(t, http.StatusNoContent, wDel.Code)

		wList := req(t, r, http.MethodGet, "/v1/transactions?year=2026&month=10", "", token)
		require.Equal(t, http.StatusOK, wList.Code)
		var list []any
		require.NoError(t, json.Unmarshal(wList.Body.Bytes(), &list))
		assert.Empty(t, list)
	})

	t.Run("audit_log entry is created on delete", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "del_audit@test.com", "Senha123")

		wCreate := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655443002","amount":"120","direction":"INCOME","date":"2026-10-10"}`,
			token)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		wDel := req(t, r, http.MethodDelete, "/v1/transactions/"+id, "", token)
		require.Equal(t, http.StatusNoContent, wDel.Code)

		var count int
		err := db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM audit_log WHERE entity = 'transaction' AND entity_id = $1 AND action = 'delete'`,
			id,
		).Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 1, count)
	})

	t.Run("non-owner gets 403", func(t *testing.T) {
		tokenA, _ := registerAndLogin(t, r, "del_ownerA@test.com", "Senha123")
		tokenB, _ := registerAndLogin(t, r, "del_ownerB@test.com", "Senha123")

		wCreate := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655443003","amount":"40","direction":"EXPENSE","date":"2026-10-01"}`,
			tokenA)
		require.Equal(t, http.StatusCreated, wCreate.Code)
		var created map[string]any
		require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &created))
		id := created["id"].(string)

		w := req(t, r, http.MethodDelete, "/v1/transactions/"+id, "", tokenB)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("unknown id returns 404", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "del_404@test.com", "Senha123")
		w := req(t, r, http.MethodDelete, "/v1/transactions/00000000-0000-0000-0000-000000000000", "", token)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("without token returns 401", func(t *testing.T) {
		w := req(t, r, http.MethodDelete, "/v1/transactions/some-id", "", "")
		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})
}

func TestHandler_Delete_InternalError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user_id", "user-test"); c.Next() })
	h := transaction.NewHandlerWithService(&mockSvc{err: fmt.Errorf("unexpected")})
	h.RegisterRoutes(r.Group("/v1"))

	httpReq := httptest.NewRequestWithContext(context.Background(),
		http.MethodDelete, "/v1/transactions/some-id", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httpReq)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Contains(t, w.Body.String(), "internal server error")
}

// --- Idempotência ---

func TestIntegration_Idempotency(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	t.Run("same client_id twice returns 200 on second call with identical response", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "idem_twice@test.com", "Senha123")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655450001","amount":"99.99","direction":"EXPENSE","date":"2026-10-01"}`

		w1 := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		require.Equal(t, http.StatusCreated, w1.Code)
		var first map[string]any
		require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &first))

		w2 := req(t, r, http.MethodPost, "/v1/transactions", body, token)
		assert.Equal(t, http.StatusOK, w2.Code)
		var second map[string]any
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &second))

		assert.Equal(t, first["id"], second["id"], "same id returned on retry")
		assert.Equal(t, first["amount"], second["amount"], "same amount returned on retry")
		assert.Equal(t, first["created_at"], second["created_at"], "created_at must not change")

		var count int
		require.NoError(t, db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM transactions WHERE client_id = '550e8400-e29b-41d4-a716-446655450001'`,
		).Scan(&count))
		assert.Equal(t, 1, count, "no duplicate in database")
	})

	t.Run("different client_ids with same data create two distinct transactions", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "idem_diff@test.com", "Senha123")

		w1 := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655450002","amount":"10","direction":"EXPENSE","date":"2026-10-01"}`,
			token)
		require.Equal(t, http.StatusCreated, w1.Code)

		w2 := req(t, r, http.MethodPost, "/v1/transactions",
			`{"client_id":"550e8400-e29b-41d4-a716-446655450003","amount":"10","direction":"EXPENSE","date":"2026-10-01"}`,
			token)
		require.Equal(t, http.StatusCreated, w2.Code)

		var r1, r2 map[string]any
		require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &r1))
		require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &r2))
		assert.NotEqual(t, r1["id"], r2["id"], "different client_ids must produce different transactions")

		var count int
		require.NoError(t, db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM transactions
			  WHERE client_id IN (
			    '550e8400-e29b-41d4-a716-446655450002',
			    '550e8400-e29b-41d4-a716-446655450003'
			  )`,
		).Scan(&count))
		assert.Equal(t, 2, count, "two separate rows in database")
	})

	t.Run("network retry simulation: same client_id three times yields one row", func(t *testing.T) {
		token, _ := registerAndLogin(t, r, "idem_retry@test.com", "Senha123")
		body := `{"client_id":"550e8400-e29b-41d4-a716-446655450004","amount":"200","direction":"INCOME","date":"2026-10-15"}`

		codes := make([]int, 3)
		for i := range codes {
			w := req(t, r, http.MethodPost, "/v1/transactions", body, token)
			codes[i] = w.Code
		}

		assert.Equal(t, http.StatusCreated, codes[0], "first call must be 201")
		assert.Equal(t, http.StatusOK, codes[1], "second call must be 200")
		assert.Equal(t, http.StatusOK, codes[2], "third call must be 200")

		var count int
		require.NoError(t, db.QueryRowContext(context.Background(),
			`SELECT COUNT(*) FROM transactions WHERE client_id = '550e8400-e29b-41d4-a716-446655450004'`,
		).Scan(&count))
		assert.Equal(t, 1, count, "exactly one row after three retries")
	})
}
