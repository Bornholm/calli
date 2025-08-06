package explorer

import (
	"net/http"

	"golang.org/x/net/webdav"
)

type Handler struct {
	fs  webdav.FileSystem
	mux *http.ServeMux
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func NewHandler(fs webdav.FileSystem) *Handler {
	handler := &Handler{
		fs:  fs,
		mux: &http.ServeMux{},
	}

	// Register routes
	handler.mux.HandleFunc("GET /", handler.serveIndex)
	return handler
}

var _ http.Handler = &Handler{}
