# Backend Foundation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bootstrap the Go portal backend with project structure, configuration, Google OAuth2 authentication, session middleware, and a health endpoint.

**Architecture:** Single Go binary using `chi` router. Configuration loaded from environment variables. Google OAuth2 with corporate domain enforcement via `hd` claim. Signed session cookies via `gorilla/sessions`. All `/api/*` routes protected by session middleware.

**Tech Stack:** Go 1.22, `github.com/go-chi/chi/v5`, `golang.org/x/oauth2`, `github.com/gorilla/sessions`, `github.com/stretchr/testify`

---

## File Map

| File | Responsibility |
|------|---------------|
| `go.mod` | Module definition |
| `cmd/portal/main.go` | Entrypoint: load config, wire server, listen |
| `internal/config/config.go` | Load + validate env vars into `Config` struct |
| `internal/config/config_test.go` | Tests for required/optional fields |
| `internal/auth/handler.go` | OAuth2 login, callback, logout HTTP handlers |
| `internal/auth/handler_test.go` | Tests for domain enforcement, session creation |
| `internal/auth/middleware.go` | Session middleware: protect routes, inject session into context |
| `internal/auth/middleware_test.go` | Tests for unauthenticated redirect, authenticated pass-through |
| `internal/server/server.go` | `Server` struct: holds deps, wires chi router |
| `internal/server/health.go` | `GET /healthz` handler |
| `internal/server/health_test.go` | Test for 200 + JSON response |
| `Makefile` | `build`, `run`, `test`, `dev` targets |
| `.env.example` | Template for required env vars |

---

## Task 1: Initialize Go module and project skeleton

**Files:**
- Create: `go.mod`
- Create: `Makefile`
- Create: `.env.example`
- Create: `cmd/portal/main.go` (stub)

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p cmd/portal internal/config internal/auth internal/server
```

- [ ] **Step 2: Initialize Go module**

```bash
go mod init github.com/emdash/kindle
```

- [ ] **Step 3: Create stub `cmd/portal/main.go`**

```go
package main

import "fmt"

func main() {
	fmt.Println("portal starting")
}
```

- [ ] **Step 4: Create `Makefile`**

```makefile
.PHONY: build run test dev

build:
	go build -o bin/portal ./cmd/portal

run: build
	./bin/portal

test:
	go test ./...

dev:
	go run ./cmd/portal
```

- [ ] **Step 5: Create `.env.example`**

```bash
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
CORPORATE_DOMAIN=yourcompany.com
ZONE_DOMAIN=local-dev.example.com
GCP_PROJECT_ID=your-gcp-project
CHART_REPO_URL=https://github.com/your-org/your-chart-repo
SESSION_SECRET=change-me-generate-with-openssl-rand-hex-32
DATA_DIR=./data
PRESETS_DIR=./presets
PORT=8080
```

- [ ] **Step 6: Verify build**

```bash
make build
```

Expected: `bin/portal` created with no errors.

- [ ] **Step 7: Commit**

```bash
git add go.mod cmd/portal/main.go Makefile .env.example
git commit -m "feat: initialize Go module and project skeleton"
```

---

## Task 2: Config loading

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/config/config_test.go
package config_test

import (
	"os"
	"testing"

	"github.com/emdash/kindle/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingRequired(t *testing.T) {
	os.Clearenv()
	_, err := config.Load()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GOOGLE_CLIENT_ID")
}

func TestLoad_AllRequired(t *testing.T) {
	os.Clearenv()
	os.Setenv("GOOGLE_CLIENT_ID", "client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	os.Setenv("CORPORATE_DOMAIN", "example.com")
	os.Setenv("ZONE_DOMAIN", "local-dev.example.com")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("CHART_REPO_URL", "https://github.com/org/repo")
	os.Setenv("SESSION_SECRET", "secret")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "client-id", cfg.GoogleClientID)
	assert.Equal(t, "./data", cfg.DataDir)     // default
	assert.Equal(t, "./presets", cfg.PresetsDir) // default
	assert.Equal(t, "8080", cfg.Port)            // default
}

func TestLoad_OverrideDefaults(t *testing.T) {
	os.Clearenv()
	os.Setenv("GOOGLE_CLIENT_ID", "client-id")
	os.Setenv("GOOGLE_CLIENT_SECRET", "client-secret")
	os.Setenv("CORPORATE_DOMAIN", "example.com")
	os.Setenv("ZONE_DOMAIN", "local-dev.example.com")
	os.Setenv("GCP_PROJECT_ID", "my-project")
	os.Setenv("CHART_REPO_URL", "https://github.com/org/repo")
	os.Setenv("SESSION_SECRET", "secret")
	os.Setenv("DATA_DIR", "/custom/data")
	os.Setenv("PORT", "9090")

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "/custom/data", cfg.DataDir)
	assert.Equal(t, "9090", cfg.Port)
}
```

- [ ] **Step 2: Add testify dependency and run test to verify it fails**

```bash
go get github.com/stretchr/testify@v1.9.0
go test ./internal/config/...
```

Expected: FAIL — `config` package not found.

- [ ] **Step 3: Write `internal/config/config.go`**

```go
package config

import (
	"fmt"
	"os"
)

type Config struct {
	GoogleClientID     string
	GoogleClientSecret string
	CorporateDomain    string
	ZoneDomain         string
	GCPProjectID       string
	ChartRepoURL       string
	SessionSecret      string
	DataDir            string
	PresetsDir         string
	Port               string
}

func Load() (*Config, error) {
	c := &Config{
		DataDir:    envOrDefault("DATA_DIR", "./data"),
		PresetsDir: envOrDefault("PRESETS_DIR", "./presets"),
		Port:       envOrDefault("PORT", "8080"),
	}
	required := []struct {
		key string
		dst *string
	}{
		{"GOOGLE_CLIENT_ID", &c.GoogleClientID},
		{"GOOGLE_CLIENT_SECRET", &c.GoogleClientSecret},
		{"CORPORATE_DOMAIN", &c.CorporateDomain},
		{"ZONE_DOMAIN", &c.ZoneDomain},
		{"GCP_PROJECT_ID", &c.GCPProjectID},
		{"CHART_REPO_URL", &c.ChartRepoURL},
		{"SESSION_SECRET", &c.SessionSecret},
	}
	for _, r := range required {
		v := os.Getenv(r.key)
		if v == "" {
			return nil, fmt.Errorf("required env var %s is not set", r.key)
		}
		*r.dst = v
	}
	return c, nil
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/config/... -v
```

Expected: all 3 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add config loading from environment variables"
```

---

## Task 3: Health endpoint

**Files:**
- Create: `internal/server/health.go`
- Create: `internal/server/health_test.go`

- [ ] **Step 1: Add chi dependency**

```bash
go get github.com/go-chi/chi/v5@v5.0.12
```

- [ ] **Step 2: Write the failing test**

```go
// internal/server/health_test.go
package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emdash/kindle/internal/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	server.HealthHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var body map[string]string
	require.NoError(t, json.NewDecoder(w.Body).Decode(&body))
	assert.Equal(t, "ok", body["status"])
}
```

- [ ] **Step 3: Run test to verify it fails**

```bash
go test ./internal/server/... -v
```

Expected: FAIL — `server` package not found.

- [ ] **Step 4: Write `internal/server/health.go`**

```go
package server

import (
	"encoding/json"
	"net/http"
)

func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/server/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/server/health.go internal/server/health_test.go go.mod go.sum
git commit -m "feat: add health endpoint"
```

---

## Task 4: Google OAuth2 handlers

**Files:**
- Create: `internal/auth/handler.go`
- Create: `internal/auth/handler_test.go`

- [ ] **Step 1: Add OAuth2 and sessions dependencies**

```bash
go get golang.org/x/oauth2@v0.21.0
go get github.com/gorilla/sessions@v1.2.2
```

- [ ] **Step 2: Write the failing tests**

```go
// internal/auth/handler_test.go
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

	// Simulate a callback where domain verification fails — we test the
	// domain check logic directly via the exported helper.
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
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
go test ./internal/auth/... -v
```

Expected: FAIL — `auth` package not found.

- [ ] **Step 4: Write `internal/auth/handler.go`**

```go
package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	SessionName    = "portal-session"
	sessionKeyEmail = "email"
	sessionKeyName  = "name"
	sessionKeyState = "oauth_state"
)

type HandlerConfig struct {
	ClientID        string
	ClientSecret    string
	RedirectURL     string
	CorporateDomain string
	SessionSecret   string
}

type Handler struct {
	oauthConfig     *oauth2.Config
	store           *sessions.CookieStore
	corporateDomain string
}

func NewHandler(cfg HandlerConfig) *Handler {
	return &Handler{
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			RedirectURL:  cfg.RedirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     google.Endpoint,
		},
		store:           sessions.NewCookieStore([]byte(cfg.SessionSecret)),
		corporateDomain: cfg.CorporateDomain,
	}
}

func (h *Handler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	state := generateState()
	session, _ := h.store.Get(r, SessionName)
	session.Values[sessionKeyState] = state
	session.Save(r, w)
	url := h.oauthConfig.AuthCodeURL(state, oauth2.SetAuthURLParam("hd", h.corporateDomain))
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *Handler) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := h.store.Get(r, SessionName)
	expectedState, _ := session.Values[sessionKeyState].(string)
	if r.URL.Query().Get("state") != expectedState {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}

	token, err := h.oauthConfig.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusInternalServerError)
		return
	}

	info, err := h.fetchUserInfo(r, token)
	if err != nil {
		http.Error(w, "failed to fetch user info", http.StatusInternalServerError)
		return
	}

	if err := h.VerifyHostedDomain(info.Email, h.corporateDomain); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	session.Values[sessionKeyEmail] = info.Email
	session.Values[sessionKeyName] = info.Name
	delete(session.Values, sessionKeyState)
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := h.store.Get(r, SessionName)
	session.Options.MaxAge = -1
	session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func (h *Handler) VerifyHostedDomain(email, domain string) error {
	suffix := "@" + domain
	for i := len(email) - 1; i >= 0; i-- {
		if email[i] == '@' {
			if email[i:] == suffix {
				return nil
			}
			return fmt.Errorf("email %s is not from domain %s", email, domain)
		}
	}
	return fmt.Errorf("email %s is not from domain %s", email, domain)
}

type userInfo struct {
	Email string `json:"email"`
	Name  string `json:"name"`
}

func (h *Handler) fetchUserInfo(r *http.Request, token *oauth2.Token) (*userInfo, error) {
	client := h.oauthConfig.Client(r.Context(), token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var info userInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

func generateState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
go test ./internal/auth/... -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/auth/handler.go internal/auth/handler_test.go go.mod go.sum
git commit -m "feat: add Google OAuth2 login/callback/logout handlers"
```

---

## Task 5: Session middleware

**Files:**
- Create: `internal/auth/middleware.go`
- Create: `internal/auth/middleware_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/auth/middleware_test.go
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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/auth/... -run TestMiddleware -v
```

Expected: FAIL — `Middleware`, `SetSession`, `SessionFromContext` not defined.

- [ ] **Step 3: Write `internal/auth/middleware.go`**

```go
package auth

import (
	"context"
	"net/http"
)

type contextKey string

const contextKeySession contextKey = "session"

type Session struct {
	Email string
	Name  string
}

func SessionFromContext(ctx context.Context) *Session {
	s, _ := ctx.Value(contextKeySession).(*Session)
	return s
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := h.store.Get(r, SessionName)
		email, ok := session.Values[sessionKeyEmail].(string)
		if !ok || email == "" {
			http.Redirect(w, r, "/auth/login", http.StatusFound)
			return
		}
		name, _ := session.Values[sessionKeyName].(string)
		ctx := context.WithValue(r.Context(), contextKeySession, &Session{
			Email: email,
			Name:  name,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// SetSession writes a valid session cookie to w. Used in tests and after
// successful OAuth2 callback.
func (h *Handler) SetSession(w http.ResponseWriter, r *http.Request, email, name string) {
	session, _ := h.store.Get(r, SessionName)
	session.Values[sessionKeyEmail] = email
	session.Values[sessionKeyName] = name
	session.Save(r, w)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/auth/... -v
```

Expected: all auth tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/auth/middleware.go internal/auth/middleware_test.go
git commit -m "feat: add session middleware with context injection"
```

---

## Task 6: Server wiring

**Files:**
- Create: `internal/server/server.go`
- Modify: `cmd/portal/main.go`

- [ ] **Step 1: Write `internal/server/server.go`**

```go
package server

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/emdash/kindle/internal/auth"
	"github.com/emdash/kindle/internal/config"
)

type Server struct {
	cfg     *config.Config
	auth    *auth.Handler
	router  chi.Router
}

func New(cfg *config.Config) *Server {
	authHandler := auth.NewHandler(auth.HandlerConfig{
		ClientID:        cfg.GoogleClientID,
		ClientSecret:    cfg.GoogleClientSecret,
		RedirectURL:     fmt.Sprintf("http://localhost:%s/auth/callback", cfg.Port),
		CorporateDomain: cfg.CorporateDomain,
		SessionSecret:   cfg.SessionSecret,
	})

	s := &Server{cfg: cfg, auth: authHandler}
	s.router = s.buildRouter()
	return s
}

func (s *Server) buildRouter() chi.Router {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Public routes
	r.Get("/healthz", HealthHandler)
	r.Get("/auth/login", s.auth.LoginHandler)
	r.Get("/auth/callback", s.auth.CallbackHandler)
	r.Get("/auth/logout", s.auth.LogoutHandler)

	// Protected API routes (populated by later plans)
	r.Group(func(r chi.Router) {
		r.Use(s.auth.Middleware)
		r.Get("/api/envs", placeholderHandler)
	})

	return r
}

func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) Addr() string {
	return ":" + s.cfg.Port
}

func placeholderHandler(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
```

- [ ] **Step 2: Update `cmd/portal/main.go`**

```go
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/emdash/kindle/internal/config"
	"github.com/emdash/kindle/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	srv := server.New(cfg)
	slog.Info("portal listening", "addr", srv.Addr())
	if err := http.ListenAndServe(srv.Addr(), srv.Handler()); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build and verify**

```bash
make build
```

Expected: compiles cleanly, `bin/portal` created.

- [ ] **Step 4: Run all tests**

```bash
make test
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/server/server.go cmd/portal/main.go go.mod go.sum
git commit -m "feat: wire chi router with auth and health routes"
```

---

## Task 7: Smoke test the running server

**Files:**
- No new files — manual verification

- [ ] **Step 1: Copy `.env.example` to `.env` and fill in test values**

For a local smoke test you can use dummy values for OAuth2 (the server will start; actual login won't work without real credentials). You must provide real values for a full end-to-end test.

```bash
cp .env.example .env
# Edit .env with real or test values
```

- [ ] **Step 2: Run the server**

```bash
set -a && source .env && set +a && make run
```

Expected output: `portal listening addr=:8080`

- [ ] **Step 3: Verify health endpoint**

```bash
curl -s http://localhost:8080/healthz
```

Expected: `{"status":"ok"}`

- [ ] **Step 4: Verify auth redirect**

```bash
curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/api/envs
```

Expected: `302`

- [ ] **Step 5: Commit**

```bash
git add .env.example
git commit -m "feat: complete backend foundation — auth, health, server wiring"
```
