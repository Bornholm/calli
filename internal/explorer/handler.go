package explorer

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/store"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

type Handler struct {
	baseURL string
	fs      webdav.FileSystem
	mux     *http.ServeMux
	store   *store.Store
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(baseURL string, fs webdav.FileSystem, store *store.Store) *Handler {
	handler := &Handler{
		baseURL: baseURL,
		fs:      fs,
		mux:     &http.ServeMux{},
		store:   store,
	}

	// Register routes
	handler.mux.HandleFunc("GET /", handler.serveIndex)
	handler.mux.HandleFunc("POST /actions/regenerate-password", handler.regeneratePassword)
	return handler
}

// regeneratePassword handles credential regeneration requests
func (h *Handler) regeneratePassword(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user from context
	authUser, err := authz.ContextUser(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "could not get user from context", log.Error(errors.WithStack(err)))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Cast to store.User
	storeUser, ok := authUser.(*store.User)
	if !ok {
		slog.ErrorContext(ctx, "user is not a store.User")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Regenerate password
	password, err := h.store.RegenerateBasicPassword(ctx, storeUser.ID, 16)
	if err != nil {
		slog.ErrorContext(ctx, "could not regenerate password", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Redirect with flash message
	redirectURL, err := url.Parse(r.Referer())
	if err != nil || redirectURL.String() == "" {
		redirectURL, _ = url.Parse("/")
	}

	q := redirectURL.Query()
	q.Set("flash", "Credentials regenerated. New password: "+password)
	redirectURL.RawQuery = q.Encode()

	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

var _ http.Handler = &Handler{}
