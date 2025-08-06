package oauth2

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/bornholm/calli/internal/authn"
	"github.com/gorilla/sessions"
	"github.com/pkg/errors"
)

type Provider struct {
	ID    string
	Label string
	Icon  string
}

type Handler struct {
	mux                *http.ServeMux
	sessionStore       sessions.Store
	sessionName        string
	providers          []Provider
	prefix             string
	postLoginRedirect  string
	postLogoutRedirect string
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(sessionStore sessions.Store, funcs ...OptionFunc) *Handler {
	opts := NewOptions(funcs...)
	h := &Handler{
		mux:                http.NewServeMux(),
		sessionStore:       sessionStore,
		sessionName:        opts.SessionName,
		providers:          opts.Providers,
		prefix:             opts.Prefix,
		postLoginRedirect:  opts.PostLoginRedirect,
		postLogoutRedirect: opts.PostLogoutRedirect,
	}

	h.mux.HandleFunc(fmt.Sprintf("GET %s/login", h.prefix), h.getLoginPage)
	h.mux.Handle(fmt.Sprintf("GET %s/providers/{provider}", h.prefix), withContextProvider(http.HandlerFunc(h.handleProvider)))
	h.mux.Handle(fmt.Sprintf("GET %s/providers/{provider}/callback", h.prefix), withContextProvider(http.HandlerFunc(h.handleProviderCallback)))
	h.mux.HandleFunc(fmt.Sprintf("GET %s/logout", h.prefix), h.handleLogout)
	h.mux.Handle(fmt.Sprintf("GET %s/providers/{provider}/logout", h.prefix), withContextProvider(http.HandlerFunc(h.handleProviderLogout)))

	return h
}

func (h *Handler) Authenticator(authoritative bool) authn.Authenticator {
	return authn.AuthenticateFunc(func(w http.ResponseWriter, r *http.Request) (authn.User, error) {
		user, err := h.retrieveSessionUser(r)
		if err != nil {
			if authoritative {
				slog.ErrorContext(r.Context(), "could not retrieve user from session", slog.Any("error", errors.WithStack(err)))
				http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
				return nil, errors.WithStack(authn.ErrCancel)
			} else {
				return nil, nil
			}
		}

		return user, nil
	})
}

var _ http.Handler = &Handler{}

func withContextProvider(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		provider := r.PathValue("provider")
		r = r.WithContext(context.WithValue(r.Context(), "provider", provider))
		h.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
