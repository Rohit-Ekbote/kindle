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
