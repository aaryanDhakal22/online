package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	keyApp "quicc/online/internal/app/key"
	"quicc/online/internal/infra/database/models"
	"quicc/online/internal/infra/database/repositories"
	"quicc/online/internal/migrations"
	"quicc/online/internal/observability"

	"github.com/labstack/echo/v5"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func newKeyServer(t *testing.T, defaultKey string) (*echo.Echo, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, migrations.Apply(context.Background(), db))

	meters, err := observability.NewMeters()
	require.NoError(t, err)
	svc := keyApp.NewKeyService(repositories.NewKeyRepository(models.New(db), zerolog.Nop()), zerolog.Nop(), meters)
	require.NoError(t, svc.Bootstrap(context.Background(), defaultKey))
	h := NewHandler(svc, nil, meters, zerolog.Nop())

	e := echo.New()
	e.GET("/api/v1/verify", h.Verify)
	e.POST("/api/v1/setKey", h.SetKey)
	e.GET("/api/v1/keys", h.ListKeys)
	e.POST("/api/v1/keys/:id/retire", h.RetireKey)
	return e, db
}

func do(e *echo.Echo, method, path, body string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body["error"]
}

func TestRetireKey_HTTPStatuses(t *testing.T) {
	e, db := newKeyServer(t, "live")
	var liveID string
	require.NoError(t, db.QueryRow(`SELECT id FROM api_keys WHERE key='live'`).Scan(&liveID))

	rec := do(e, "POST", "/api/v1/keys/"+liveID+"/retire", "", nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "last_active_key", errCode(t, rec))

	rec = do(e, "POST", "/api/v1/keys/nope/retire", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "not_found", errCode(t, rec))

	rec = do(e, "POST", "/api/v1/setKey", `{"key":"next"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = do(e, "POST", "/api/v1/keys/"+liveID+"/retire", "", nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "new_key_unconfirmed", errCode(t, rec))

	rec = do(e, "GET", "/api/v1/verify", "", map[string]string{"Authorization": "Bearer next"})
	require.Equal(t, http.StatusOK, rec.Code)

	rec = do(e, "POST", "/api/v1/keys/"+liveID+"/retire", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = do(e, "POST", "/api/v1/keys/"+liveID+"/retire", "", nil)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, "not_active", errCode(t, rec))

	rec = do(e, "GET", "/api/v1/keys", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), `"live"`)
	assert.NotContains(t, rec.Body.String(), `"next"`)
	var list struct {
		Keys []keyApp.KeyInfo `json:"keys"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Keys, 2)
	assert.Equal(t, "active", list.Keys[0].Status)
	assert.Equal(t, "inactive", list.Keys[1].Status)
}
