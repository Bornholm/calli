package explorer

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bornholm/calli/internal/ui"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
)

// serveIndex handles requests to browse directories and serve files
func (h *Handler) serveIndex(w http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path

	// Normalize path for file system operations
	fsPath := path.Clean(urlPath)
	if fsPath == "." {
		fsPath = "/"
	}

	ctx := r.Context()

	// Open the directory
	dirFile, err := h.fs.OpenFile(ctx, fsPath, os.O_RDONLY, 0)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		slog.ErrorContext(ctx, "could not open directory", log.Error(errors.WithStack(err)), slog.String("path", fsPath))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer dirFile.Close()

	// Get file info to check if it's a directory
	fileInfo, err := dirFile.Stat()
	if err != nil {
		slog.ErrorContext(ctx, "could not stat file", log.Error(errors.WithStack(err)), slog.String("path", fsPath))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// If it's not a directory, serve the file
	if !fileInfo.IsDir() {
		// Not a directory, redirect to WebDAV
		http.Redirect(w, r, "/dav"+urlPath, http.StatusFound)
		return
	}

	// Get explorer data for the directory
	data := h.getExplorerData(ctx, fsPath)

	// Render the full template
	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		slog.ErrorContext(ctx, "could not execute template", log.Error(errors.WithStack(err)))
		return
	}
}

// getExplorerData retrieves directory contents and creates template data
func (h *Handler) getExplorerData(ctx context.Context, fsPath string) FileExplorerTemplateData {
	// Default to empty data structure
	data := FileExplorerTemplateData{
		Path:            fsPath,
		ParentPath:      "/",
		BreadcrumbItems: []string{},
		Directories:     []FileTemplateData{},
		Files:           []FileTemplateData{},
	}

	// Open the directory
	dirFile, err := h.fs.OpenFile(ctx, fsPath, os.O_RDONLY, 0)
	if err != nil {
		slog.ErrorContext(ctx, "could not open directory", log.Error(errors.WithStack(err)), slog.String("path", fsPath))
		return data
	}
	defer dirFile.Close()

	// Get file info
	fileInfo, err := dirFile.Stat()
	if err != nil {
		slog.ErrorContext(ctx, "could not stat file", log.Error(errors.WithStack(err)), slog.String("path", fsPath))
		return data
	}

	// Make sure it's a directory
	if !fileInfo.IsDir() {
		return data
	}

	// List directory contents
	files, err := dirFile.Readdir(-1)
	if err != nil {
		slog.ErrorContext(ctx, "could not read directory", log.Error(errors.WithStack(err)), slog.String("path", fsPath))
		return data
	}

	// Create template data
	dirs := []FileTemplateData{}
	regularFiles := []FileTemplateData{}

	for _, file := range files {
		// Skip hidden files
		if strings.HasPrefix(file.Name(), ".") {
			continue
		}

		fileData := NewFileTemplateData(file, fsPath)

		if file.IsDir() {
			dirs = append(dirs, fileData)
		} else {
			regularFiles = append(regularFiles, fileData)
		}
	}

	// Sort directories and files alphabetically
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].Name < dirs[j].Name
	})
	sort.Slice(regularFiles, func(i, j int) bool {
		return regularFiles[i].Name < regularFiles[j].Name
	})

	// Calculate parent path for navigation
	parentPath := "/"
	if fsPath != "/" {
		parentPath = filepath.Dir(fsPath)
		if parentPath == "." {
			parentPath = "/"
		}
	}

	// Generate breadcrumb items
	breadcrumbs := []string{}
	if fsPath != "/" {
		parts := strings.Split(strings.Trim(fsPath, "/"), "/")
		currentPath := ""
		for _, part := range parts {
			currentPath = path.Join(currentPath, part)
			breadcrumbs = append(breadcrumbs, part)
		}
	}

	data = FileExplorerTemplateData{
		HeadTemplateData: ui.HeadTemplateData{
			PageTitle: fsPath,
		},
		Path:            fsPath,
		ParentPath:      parentPath,
		BreadcrumbItems: breadcrumbs,
		Directories:     dirs,
		Files:           regularFiles,
	}

	return data
}
