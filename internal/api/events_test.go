package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/api"
	"github.com/emdash/kindle/internal/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventsHandler_List(t *testing.T) {
	d := openTestDB(t)
	d.WriteEvent(db.Event{
		EnvName: "dev-alice", ActorEmail: "alice@example.com",
		ActionType: "create", Description: "Created env", Outcome: db.OutcomeSuccess,
	})

	h := api.NewEventsHandler(api.EventsHandlerDeps{DB: d})
	r := chi.NewRouter()
	r.Get("/api/envs/{name}/events", h.List)

	req := httptest.NewRequest(http.MethodGet, "/api/envs/dev-alice/events", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var body []map[string]any
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Len(t, body, 1)
	assert.Equal(t, "create", body[0]["action_type"])
}
