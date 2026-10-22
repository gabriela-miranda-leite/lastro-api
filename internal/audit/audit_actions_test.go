package audit_test

// Integration tests that verify audit_log entries are created (or not) by
// destructive actions across different domains, and that the audit_log is
// immutable via the API.

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gabriela-miranda-leite/lastro-api/internal/audit"
	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
	"github.com/gabriela-miranda-leite/lastro-api/internal/couple"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/testhelpers"
	"github.com/gabriela-miranda-leite/lastro-api/internal/transaction"
	"github.com/gabriela-miranda-leite/lastro-api/internal/user"
)

func setupFullRouter(db *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	authSvc := auth.NewService(auth.NewRepository(db), testJWTSecret)
	authHandler := auth.NewHandler(authSvc)
	v1 := r.Group("/v1")
	authHandler.RegisterRoutes(v1.Group("/auth"))

	protected := v1.Group("/")
	protected.Use(middleware.Auth(db, testJWTSecret))
	authHandler.RegisterProtectedRoutes(protected)

	audit.NewHandler(audit.NewService(audit.NewRepository(db))).RegisterRoutes(protected)
	transaction.NewHandler(transaction.NewService(transaction.NewRepository(db))).RegisterRoutes(protected)
	couple.NewHandler(couple.NewService(couple.NewRepository(db))).RegisterRoutes(protected)
	user.NewHandler(user.NewService(user.NewRepository(db))).RegisterRoutes(protected)

	return r
}

func countAudit(t *testing.T, db *sql.DB, userID, action string) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE user_id = $1 AND action = $2`,
		userID, action,
	).Scan(&n))
	return n
}

func formCoupleActions(t *testing.T, r *gin.Engine, emailA, emailB string) (tokenA, idA, tokenB, idB string) {
	t.Helper()
	tokenA, idA = registerAndLogin(t, r, emailA, "Senha123")
	tokenB, idB = registerAndLogin(t, r, emailB, "Senha123")

	w := doReq(t, r, http.MethodPost, "/v1/couples/invite", "", tokenA)
	require.Equal(t, http.StatusCreated, w.Code)
	var inv map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &inv))

	w = doReq(t, r, http.MethodPost, "/v1/couples/join",
		fmt.Sprintf(`{"code":%q}`, inv["code"].(string)), tokenB)
	require.Equal(t, http.StatusOK, w.Code)
	return
}

// --- delete_transaction ---

func TestAudit_DeleteTransaction_CreatesEntry(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	wc := doReq(t, r, http.MethodPost, "/v1/transactions",
		`{"client_id":"a0000001-0000-0000-0000-000000000001","amount":"200.00","direction":"EXPENSE","date":"2026-10-01"}`,
		token)
	require.Equal(t, http.StatusCreated, wc.Code)
	var tx map[string]any
	require.NoError(t, json.Unmarshal(wc.Body.Bytes(), &tx))
	txID := tx["id"].(string)

	wd := doReq(t, r, http.MethodDelete, "/v1/transactions/"+txID, "", token)
	require.Equal(t, http.StatusNoContent, wd.Code)

	assert.Equal(t, 1, countAudit(t, db, userID, "delete"))

	// verify metadata contains the transaction snapshot
	var meta json.RawMessage
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT metadata FROM audit_log WHERE user_id = $1 AND action = 'delete' AND entity_id = $2`,
		userID, txID,
	).Scan(&meta))
	var m map[string]any
	require.NoError(t, json.Unmarshal(meta, &m))
	assert.Equal(t, "200.00", m["amount"])
	assert.Equal(t, "2026-10-01", m["date"])
}

// --- unlink_couple ---

func TestAudit_UnlinkCouple_CreatesTwoEntries(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	tokenA, idA, _, idB := formCoupleActions(t, r, uniqueEmail(t, "a"), uniqueEmail(t, "b"))

	wd := doReq(t, r, http.MethodDelete, "/v1/couples", "", tokenA)
	require.Equal(t, http.StatusNoContent, wd.Code)

	assert.Equal(t, 1, countAudit(t, db, idA, "unlink_couple"), "A should have 1 entry")
	assert.Equal(t, 1, countAudit(t, db, idB, "unlink_couple"), "B should have 1 entry")
}

// --- export_data ---

func TestAudit_ExportData_CreatesEntry(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	w := doReq(t, r, http.MethodGet, "/v1/users/me/export", "", token)
	require.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, 1, countAudit(t, db, userID, "export_data"))
}

// --- rollback: action fails → no audit_log entry ---

func TestAudit_DeleteTransaction_RollbackOnFailure(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	// Try to delete a non-existent transaction → 404, no audit entry
	w := doReq(t, r, http.MethodDelete,
		"/v1/transactions/00000000-0000-0000-0000-000000000000", "", token)
	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.Equal(t, 0, countAudit(t, db, userID, "delete"))
}

func TestAudit_UnlinkCouple_RollbackWhenNotInCouple(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	// User has no couple → unlink returns 404, no audit entries
	w := doReq(t, r, http.MethodDelete, "/v1/couples", "", token)
	assert.Equal(t, http.StatusNotFound, w.Code)

	assert.Equal(t, 0, countAudit(t, db, userID, "unlink_couple"))
}

// --- immutability: PUT/PATCH/DELETE on /audit → 405 ---

func TestAudit_Immutability_PutReturns405(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	insertAuditEntry(t, db, userID, "export_data")

	w := doReq(t, r, http.MethodPut, "/v1/audit", `{}`, token)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAudit_Immutability_PatchReturns405(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	token, _ := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	w := doReq(t, r, http.MethodPatch, "/v1/audit", `{}`, token)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestAudit_Immutability_DeleteReturns405(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupFullRouter(db)

	token, _ := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	w := doReq(t, r, http.MethodDelete, "/v1/audit", "", token)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
}
