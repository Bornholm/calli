package oauth2

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/markbates/goth/gothic"
	"github.com/pkg/errors"
)

func (h *Handler) handleProvider(w http.ResponseWriter, r *http.Request) {
	if _, err := gothic.CompleteUserAuth(w, r); err == nil {
		http.Redirect(w, r, fmt.Sprintf("%s/logout", h.prefix), http.StatusTemporaryRedirect)
	} else {
		gothic.BeginAuthHandler(w, r)
	}
}

func (h *Handler) handleProviderCallback(w http.ResponseWriter, r *http.Request) {
	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		slog.ErrorContext(r.Context(), "could not complete user auth", slog.Any("error", errors.WithStack(err)))
		http.Redirect(w, r, fmt.Sprintf("%s/logout", h.prefix), http.StatusTemporaryRedirect)
		return
	}

	ctx := r.Context()

	slog.DebugContext(ctx, "authenticated user", slog.Any("user", gothUser))

	user := &User{
		Subject:  gothUser.UserID,
		Provider: gothUser.Provider,

		Nickname: gothUser.Name,
		Email:    gothUser.Email,

		AccessToken: gothUser.AccessToken,
		IDToken:     gothUser.IDToken,
	}

	if user.Email == "" {
		slog.ErrorContext(r.Context(), "could not authenticate user", slog.Any("error", errors.New("user email missing")))
		http.Redirect(w, r, fmt.Sprintf("%s/logout", h.prefix), http.StatusTemporaryRedirect)
		return
	}

	if user.UserProvider() == "" {
		slog.ErrorContext(r.Context(), "could not authenticate user", slog.Any("error", errors.New("user provider missing")))
		http.Redirect(w, r, fmt.Sprintf("%s/logout", h.prefix), http.StatusTemporaryRedirect)
		return
	}

	rawPreferredUsername, exists := gothUser.RawData["preferred_username"]
	if exists {
		if preferredUsername, ok := rawPreferredUsername.(string); ok {
			user.Nickname = preferredUsername
		}
	}

	if err := h.storeSessionUser(w, r, user); err != nil {
		slog.ErrorContext(r.Context(), "could not store session user", slog.Any("error", errors.WithStack(err)))
		http.Redirect(w, r, fmt.Sprintf("%s/logout", h.prefix), http.StatusTemporaryRedirect)
		return
	}

	http.Redirect(w, r, h.postLoginRedirect, http.StatusSeeOther)
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	user, err := h.retrieveSessionUser(r)
	if err != nil && !errors.Is(err, errSessionNotFound) {
		log.Printf("[ERROR] %+v", errors.WithStack(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if err := h.clearSession(w, r); err != nil && !errors.Is(err, errSessionNotFound) {
		log.Printf("[ERROR] %+v", errors.WithStack(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if user == nil {
		http.Redirect(w, r, h.postLogoutRedirect, http.StatusTemporaryRedirect)
		return
	}

	redirectURL := fmt.Sprintf("%s/providers/%s/logout", h.prefix, user.UserProvider())

	http.Redirect(w, r, redirectURL, http.StatusTemporaryRedirect)
}

func (h *Handler) handleProviderLogout(w http.ResponseWriter, r *http.Request) {
	if err := gothic.Logout(w, r); err != nil {
		log.Printf("[ERROR] %+v", errors.WithStack(err))
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.postLogoutRedirect, http.StatusTemporaryRedirect)
}
