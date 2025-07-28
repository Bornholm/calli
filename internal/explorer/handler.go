package explorer

import (
	"net/http"
	"path"
	"strings"

	"log/slog"

	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
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

// servePartialIndex serves just the file listing part of the page for HTMX requests
func (h *Handler) servePartialIndex(w http.ResponseWriter, r *http.Request) {
	// Only respond to HTMX requests
	if r.Header.Get("HX-Request") != "true" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	urlPath := r.URL.Path
	urlPath = strings.TrimPrefix(urlPath, "/partial")

	// Normalize path for file system operations
	fsPath := path.Clean(urlPath)
	if fsPath == "." {
		fsPath = "/"
	}

	// Set the current path for the browser URL (HTMX pushes this to history)
	w.Header().Set("HX-Push-Url", fsPath)

	// Render the partial template
	if err := templates.ExecuteTemplate(w, "file-list", h.getExplorerData(r.Context(), fsPath)); err != nil {
		slog.ErrorContext(r.Context(), "could not execute partial template", log.Error(errors.WithStack(err)))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

var _ http.Handler = &Handler{}
