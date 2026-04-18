package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emdash/kindle/internal/auth"
	"github.com/stretchr/testify/assert"
)

func newTestHandler() *auth.Handler {
	return auth.NewHandler(auth.HandlerConfig{
		ClientID:        "test-client-id",
		ClientSecret:    "test-client-secret",
		RedirectURL:     "http://localhost:8080/auth/callback",
		CorporateDomain: "example.com",
		SessionSecret:   "test-secret-that-is-long-enough-32b",
	})
}

func TestLoginHandler_Redirects(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/login", nil)
	w := httptest.NewRecorder()

	h.LoginHandler(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	loc := w.Header().Get("Location")
	assert.Contains(t, loc, "accounts.google.com")
	assert.Contains(t, loc, "test-client-id")
}

func TestCallbackHandler_WrongDomain(t *testing.T) {
	h := newTestHandler()

	err := h.VerifyHostedDomain("outsider@otherdomain.com", "example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "outsider@otherdomain.com")
}

func TestVerifyHostedDomain_Match(t *testing.T) {
	h := newTestHandler()
	err := h.VerifyHostedDomain("engineer@example.com", "example.com")
	assert.NoError(t, err)
}

func TestLogoutHandler_ClearsSession(t *testing.T) {
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/auth/logout", nil)
	w := httptest.NewRecorder()

	h.LogoutHandler(w, req)

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))
}
