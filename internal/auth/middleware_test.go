package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emdash/kindle/internal/auth"
	"github.com/stretchr/testify/assert"
)

func TestMiddleware_UnauthenticatedRedirects(t *testing.T) {
	h := newTestHandler()
	protected := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/envs", nil)
	w := httptest.NewRecorder()

	protected.ServeHTTP(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/auth/login", w.Header().Get("Location"))
}

func TestMiddleware_AuthenticatedPasses(t *testing.T) {
	h := newTestHandler()

	// Build a request with a valid session cookie.
	req := httptest.NewRequest(http.MethodGet, "/api/envs", nil)
	w := httptest.NewRecorder()
	h.SetSession(w, req, "engineer@example.com", "Engineer Name")
	cookie := w.Result().Cookies()[0]

	// Now make the protected request with that cookie.
	req2 := httptest.NewRequest(http.MethodGet, "/api/envs", nil)
	req2.AddCookie(cookie)
	w2 := httptest.NewRecorder()

	reached := false
	protected := h.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		sess := auth.SessionFromContext(r.Context())
		assert.Equal(t, "engineer@example.com", sess.Email)
		w.WriteHeader(http.StatusOK)
	}))
	protected.ServeHTTP(w2, req2)

	assert.True(t, reached)
	assert.Equal(t, http.StatusOK, w2.Code)
}
