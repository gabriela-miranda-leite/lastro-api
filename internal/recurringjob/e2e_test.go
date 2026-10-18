package recurringjob_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/recurring"
	"github.com/gabriela-miranda-leite/lastro-api/internal/recurringjob"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
)

const e2eJWTSecret = "e2e-secret"

func doE2E(t *testing.T, r *gin.Engine, method, path, body, token string) *httptest.ResponseRecorder {
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

func TestE2E_Recurring_FullFlow(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), e2eJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))
	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, e2eJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)
	recurring.NewHandler(recurring.NewService(recurring.NewRepository(db))).RegisterRoutes(protected)

	job := recurringjob.New(db)

	// 1. registrar usuário
	w := doE2E(t, r, http.MethodPost, "/v1/auth/register",
		`{"name":"E2E","email":"e2e_full@test.com","password":"Senha123"}`, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var loginResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loginResp))
	token := loginResp["access_token"].(string)
	userID := loginResp["user"].(map[string]any)["id"].(string)

	// 2. criar fixo via HTTP
	w = doE2E(t, r, http.MethodPost, "/v1/recurring",
		`{"name":"Aluguel","amount":1500,"due_day":5}`, token)
	require.Equal(t, http.StatusCreated, w.Code)
	var rec map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &rec))
	recID := rec["id"].(string)

	// 3. rodar o job — deve criar 1 transação com valor original
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	var txCount int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&txCount))
	assert.Equal(t, 1, txCount)

	var txAmount, txDesc string
	var txDate time.Time
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT amount::text, description, date FROM transactions
		  WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&txAmount, &txDesc, &txDate))
	assert.Equal(t, "1500.00", txAmount)
	assert.Equal(t, "Aluguel", txDesc)
	assert.Equal(t, 5, txDate.Day())

	// 4. criar override amount via HTTP → valor diferente em novembro
	body := `{"year":2026,"month":11,"amount_override":1650,"active":true}`
	w = doE2E(t, r, http.MethodPut, "/v1/recurring/"+recID+"/override", body, token)
	require.Equal(t, http.StatusOK, w.Code)

	require.NoError(t, job.Run(context.Background(), 2026, time.November))

	var novTxAmount string
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT amount::text FROM transactions
		  WHERE user_id = $1 AND deleted_at IS NULL AND date >= '2026-11-01'`, userID,
	).Scan(&novTxAmount))
	assert.Equal(t, "1650.00", novTxAmount)

	// 5. criar override active=false via HTTP → dezembro não gera transação
	w = doE2E(t, r, http.MethodPut, "/v1/recurring/"+recID+"/override",
		`{"year":2026,"month":12,"active":false}`, token)
	require.Equal(t, http.StatusOK, w.Code)

	require.NoError(t, job.Run(context.Background(), 2026, time.December))

	var totalTx int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&totalTx))
	assert.Equal(t, 2, totalTx) // outubro + novembro, dezembro foi pulado
}

func TestE2E_Recurring_JobIdempotent_AfterHTTP(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), e2eJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))
	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, e2eJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)
	recurring.NewHandler(recurring.NewService(recurring.NewRepository(db))).RegisterRoutes(protected)

	job := recurringjob.New(db)

	w := doE2E(t, r, http.MethodPost, "/v1/auth/register",
		`{"name":"E2E","email":"e2e_idem@test.com","password":"Senha123"}`, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var loginResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loginResp))
	token := loginResp["access_token"].(string)
	userID := loginResp["user"].(map[string]any)["id"].(string)

	doE2E(t, r, http.MethodPost, "/v1/recurring",
		`{"name":"Netflix","amount":40,"due_day":15}`, token)

	require.NoError(t, job.Run(context.Background(), 2026, time.October))
	require.NoError(t, job.Run(context.Background(), 2026, time.October))
	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	var count int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&count))
	assert.Equal(t, 1, count)
}

func TestE2E_Recurring_DeactivateViaHTTP_ThenJobSkips(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), e2eJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))
	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, e2eJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)
	recurring.NewHandler(recurring.NewService(recurring.NewRepository(db))).RegisterRoutes(protected)

	job := recurringjob.New(db)

	w := doE2E(t, r, http.MethodPost, "/v1/auth/register",
		`{"name":"E2E","email":"e2e_deact@test.com","password":"Senha123"}`, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var loginResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loginResp))
	token := loginResp["access_token"].(string)
	userID := loginResp["user"].(map[string]any)["id"].(string)

	// criar fixo
	wCreate := doE2E(t, r, http.MethodPost, "/v1/recurring",
		`{"name":"Gym","amount":80,"due_day":1}`, token)
	require.Equal(t, http.StatusCreated, wCreate.Code)
	var rec map[string]any
	require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &rec))
	recID := rec["id"].(string)

	// desativar o próprio recorrente via PUT /recurring/:id
	doE2E(t, r, http.MethodPut, "/v1/recurring/"+recID, `{"active":false}`, token)

	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	var count int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestE2E_Recurring_DeleteViaHTTP_ThenJobIgnores(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), e2eJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))
	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, e2eJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)
	recurring.NewHandler(recurring.NewService(recurring.NewRepository(db))).RegisterRoutes(protected)

	job := recurringjob.New(db)

	w := doE2E(t, r, http.MethodPost, "/v1/auth/register",
		`{"name":"E2E","email":"e2e_del@test.com","password":"Senha123"}`, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var loginResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loginResp))
	token := loginResp["access_token"].(string)
	userID := loginResp["user"].(map[string]any)["id"].(string)

	wCreate := doE2E(t, r, http.MethodPost, "/v1/recurring",
		`{"name":"Temp","amount":50,"due_day":10}`, token)
	require.Equal(t, http.StatusCreated, wCreate.Code)
	var rec map[string]any
	require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &rec))
	recID := rec["id"].(string)

	// deletar via HTTP antes do job rodar
	wDel := doE2E(t, r, http.MethodDelete, "/v1/recurring/"+recID, "", token)
	require.Equal(t, http.StatusNoContent, wDel.Code)

	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	var count int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestE2E_Recurring_UpdateAmountViaHTTP_ThenJobUsesNewAmount(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), e2eJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))
	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, e2eJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)
	recurring.NewHandler(recurring.NewService(recurring.NewRepository(db))).RegisterRoutes(protected)

	job := recurringjob.New(db)

	w := doE2E(t, r, http.MethodPost, "/v1/auth/register",
		`{"name":"E2E","email":"e2e_upd@test.com","password":"Senha123"}`, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var loginResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loginResp))
	token := loginResp["access_token"].(string)
	userID := loginResp["user"].(map[string]any)["id"].(string)

	wCreate := doE2E(t, r, http.MethodPost, "/v1/recurring",
		`{"name":"Internet","amount":100,"due_day":5}`, token)
	require.Equal(t, http.StatusCreated, wCreate.Code)
	var rec map[string]any
	require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &rec))
	recID := rec["id"].(string)

	// atualizar valor permanente via PUT /recurring/:id
	doE2E(t, r, http.MethodPut, "/v1/recurring/"+recID, `{"amount":120}`, token)

	require.NoError(t, job.Run(context.Background(), 2026, time.October))

	var txAmount string
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT amount::text FROM transactions WHERE user_id = $1 AND deleted_at IS NULL`, userID,
	).Scan(&txAmount))
	assert.Equal(t, "120.00", txAmount)
}

func TestE2E_Recurring_OverrideDoesNotAffectOtherMonths(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), e2eJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))
	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, e2eJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)
	recurring.NewHandler(recurring.NewService(recurring.NewRepository(db))).RegisterRoutes(protected)

	job := recurringjob.New(db)

	w := doE2E(t, r, http.MethodPost, "/v1/auth/register",
		`{"name":"E2E","email":"e2e_months@test.com","password":"Senha123"}`, "")
	require.Equal(t, http.StatusCreated, w.Code)
	var loginResp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &loginResp))
	token := loginResp["access_token"].(string)
	userID := loginResp["user"].(map[string]any)["id"].(string)

	wCreate := doE2E(t, r, http.MethodPost, "/v1/recurring",
		`{"name":"Aluguel","amount":2000,"due_day":1}`, token)
	require.Equal(t, http.StatusCreated, wCreate.Code)
	var rec map[string]any
	require.NoError(t, json.Unmarshal(wCreate.Body.Bytes(), &rec))
	recID := rec["id"].(string)

	// override só em outubro
	doE2E(t, r, http.MethodPut, "/v1/recurring/"+recID+"/override",
		`{"year":2026,"month":10,"amount_override":2500,"active":true}`, token)

	require.NoError(t, job.Run(context.Background(), 2026, time.October))
	require.NoError(t, job.Run(context.Background(), 2026, time.November))
	require.NoError(t, job.Run(context.Background(), 2026, time.December))

	rows, err := db.QueryContext(context.Background(),
		`SELECT amount::text, date FROM transactions
		  WHERE user_id = $1 AND deleted_at IS NULL ORDER BY date`, userID)
	require.NoError(t, err)
	defer rows.Close() //nolint:errcheck // read-only query

	type txRow struct {
		amount string
		month  time.Month
	}
	var txs []txRow
	for rows.Next() {
		var row txRow
		var date time.Time
		require.NoError(t, rows.Scan(&row.amount, &date))
		row.month = date.Month()
		txs = append(txs, row)
	}
	require.NoError(t, rows.Err())

	require.Len(t, txs, 3)
	assert.Equal(t, "2500.00", txs[0].amount) // outubro — override
	assert.Equal(t, "2000.00", txs[1].amount) // novembro — original
	assert.Equal(t, "2000.00", txs[2].amount) // dezembro — original

}
