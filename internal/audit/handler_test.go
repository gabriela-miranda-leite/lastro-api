package audit_test

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

	"github.com/gabriela-miranda-leite/lastro-api/internal/audit"
	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
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

	audit.NewHandler(audit.NewService(audit.NewRepository(db))).RegisterRoutes(protected)

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

func insertAuditEntry(t *testing.T, db *sql.DB, userID, action string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO audit_log (user_id, action, entity, entity_id, metadata)
		 VALUES ($1, $2, 'user', $1, '{"test":true}')`,
		userID, action,
	)
	require.NoError(t, err)
}

func getAudit(t *testing.T, r *gin.Engine, token, query string) map[string]any {
	t.Helper()
	path := "/v1/audit"
	if query != "" {
		path += "?" + query
	}
	w := doReq(t, r, http.MethodGet, path, "", token)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// --- Tests ---

func TestIntegration_Audit_EmptyList(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, _ := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	resp := getAudit(t, r, token, "")

	assert.Equal(t, float64(0), resp["total"])
	assert.Equal(t, float64(20), resp["limit"])
	assert.Equal(t, float64(0), resp["offset"])
	data := resp["data"].([]any)
	assert.Empty(t, data)
}

func TestIntegration_Audit_ListEntries(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	insertAuditEntry(t, db, userID, "delete_transaction")
	insertAuditEntry(t, db, userID, "export_data")

	resp := getAudit(t, r, token, "")

	assert.Equal(t, float64(2), resp["total"])
	data := resp["data"].([]any)
	require.Len(t, data, 2)

	// ordered desc — export_data inserted last so appears first
	first := data[0].(map[string]any)
	assert.Equal(t, "export_data", first["action"])
	assert.NotEmpty(t, first["id"])
	assert.NotEmpty(t, first["created_at"])
	// metadata must be an object, not a string
	assert.IsType(t, map[string]any{}, first["metadata"])
}

func TestIntegration_Audit_Pagination(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	for i := range 5 {
		insertAuditEntry(t, db, userID, fmt.Sprintf("action_%d", i))
	}

	resp := getAudit(t, r, token, "limit=2&offset=0")
	assert.Equal(t, float64(5), resp["total"])
	assert.Equal(t, float64(2), resp["limit"])
	assert.Equal(t, float64(0), resp["offset"])
	assert.Len(t, resp["data"].([]any), 2)

	resp2 := getAudit(t, r, token, "limit=2&offset=2")
	assert.Equal(t, float64(5), resp2["total"])
	assert.Len(t, resp2["data"].([]any), 2)

	resp3 := getAudit(t, r, token, "limit=2&offset=4")
	assert.Len(t, resp3["data"].([]any), 1) // last page
}

func TestIntegration_Audit_IsolationBetweenUsers(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	tokenA, idA := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	tokenB, idB := registerAndLogin(t, r, uniqueEmail(t, "b"), "Senha123")

	insertAuditEntry(t, db, idA, "delete_transaction")
	insertAuditEntry(t, db, idB, "export_data")

	respA := getAudit(t, r, tokenA, "")
	assert.Equal(t, float64(1), respA["total"])

	respB := getAudit(t, r, tokenB, "")
	assert.Equal(t, float64(1), respB["total"])

	// A's entry is delete_transaction, not export_data
	dataA := respA["data"].([]any)
	assert.Equal(t, "delete_transaction", dataA[0].(map[string]any)["action"])
}

func TestIntegration_Audit_LimitOver100(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, _ := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")
	w := doReq(t, r, http.MethodGet, "/v1/audit?limit=101", "", token)
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestIntegration_Audit_NoToken(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	w := doReq(t, r, http.MethodGet, "/v1/audit", "", "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestIntegration_Audit_MetadataIsObject(t *testing.T) {
	db := testhelpers.SetupTestDB(t)
	r := setupRouter(db)

	token, userID := registerAndLogin(t, r, uniqueEmail(t, "a"), "Senha123")

	// entry with NULL metadata
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO audit_log (user_id, action, entity, entity_id) VALUES ($1, 'delete_account', 'user', $1)`,
		userID,
	)
	require.NoError(t, err)

	resp := getAudit(t, r, token, "")
	data := resp["data"].([]any)
	require.Len(t, data, 1)

	// metadata with NULL in DB should come back as JSON null, not a string
	meta := data[0].(map[string]any)["metadata"]
	assert.Nil(t, meta)
}
