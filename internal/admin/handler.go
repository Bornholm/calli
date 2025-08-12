package admin

import (
	"fmt"
	"net/http"

	"github.com/bornholm/calli/internal/store"
)

type Handler struct {
	prefix string
	store  *store.Store
	mux    *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(prefix string, store *store.Store) *Handler {
	handler := &Handler{
		prefix: prefix,
		store:  store,
		mux:    &http.ServeMux{},
	}

	// Register routes
	handler.mux.HandleFunc(fmt.Sprintf("GET %s/", prefix), handler.serveIndex)
	handler.mux.HandleFunc(fmt.Sprintf("GET %s/users", prefix), handler.serveUsers)
	handler.mux.HandleFunc(fmt.Sprintf("GET %s/groups", prefix), handler.serveGroups)

	// User CRUD routes
	handler.mux.HandleFunc(fmt.Sprintf("GET %s/users/{id}/edit", prefix), handler.serveEditUser)
	handler.mux.HandleFunc(fmt.Sprintf("POST %s/users/{id}/edit", prefix), handler.serveUpdateUser)
	handler.mux.HandleFunc(fmt.Sprintf("GET %s/users/{id}/delete", prefix), handler.serveDeleteUser)
	handler.mux.HandleFunc(fmt.Sprintf("POST %s/users/{id}/delete", prefix), handler.serveDeleteUserConfirm)

	return handler
}

var _ http.Handler = &Handler{}
