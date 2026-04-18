package api

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/emdash/kindle/internal/db"
)

type EventsHandlerDeps struct {
	DB *db.DB
}

type EventsHandler struct {
	deps EventsHandlerDeps
}

func NewEventsHandler(deps EventsHandlerDeps) *EventsHandler {
	return &EventsHandler{deps: deps}
}

func (h *EventsHandler) List(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	events, err := h.deps.DB.ListEvents(name, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func queryInt(r *http.Request, key string, def int) int {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return def
	}
	return n
}
