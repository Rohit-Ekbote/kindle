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
	SessionName     = "portal-session"
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
