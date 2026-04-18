package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/presets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testPresetsDir() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "presets", "testdata")
}

func TestTemplatesHandler_List(t *testing.T) {
	loader := presets.NewLoader(filepath.Join(filepath.Dir(testPresetsDir()), "testdata"))
	h := api.NewTemplatesHandler(api.TemplatesHandlerDeps{
		Loader: loader,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/templates", nil)
	w := httptest.NewRecorder()
	h.List(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body, 1)
	assert.Equal(t, "small-dev", body[0]["name"])
}
