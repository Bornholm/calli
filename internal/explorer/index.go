package explorer

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bornholm/calli/internal/authz"
	"github.com/bornholm/calli/internal/store"
	"github.com/bornholm/calli/internal/ui"
	"github.com/bornholm/calli/pkg/log"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
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

		if errors.Is(err, os.ErrPermission) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
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
	data := h.getExplorerData(ctx, dirFile, fileInfo)

	// Check for flash message in query parameters
	if flashMsg := r.URL.Query().Get("flash"); flashMsg != "" {
		data.FlashMessage = flashMsg
	}

	// Render the full template
	if err := templates.ExecuteTemplate(w, "index", data); err != nil {
		slog.ErrorContext(ctx, "could not execute template", log.Error(errors.WithStack(err)))
		return
	}
}

// getExplorerData retrieves directory contents and creates template data
func (h *Handler) getExplorerData(ctx context.Context, dirFile webdav.File, fileInfo fs.FileInfo) FileExplorerTemplateData {
	// Default to empty data structure
	data := FileExplorerTemplateData{
		NavbarTemplateData: ui.NavbarTemplateData{
			NavbarItems: []ui.NavbarItem{ui.NavbarItemLogout},
			Username:    "",
		},
		Path:            fileInfo.Name(),
		ParentPath:      "/",
		BreadcrumbItems: []string{},
		Directories:     []FileTemplateData{},
		Files:           []FileTemplateData{},
		IsAdmin:         false,
		WebDAVURL:       "",
	}

	// Get user from context
	authUser, err := authz.ContextUser(ctx)
	if err == nil {
		// User found in context - set admin status
		storeUser, ok := authUser.(*store.User)
		if ok {
			data.IsAdmin = storeUser.IsAdmin
			if storeUser.Nickname != "" {
				data.Username = storeUser.Nickname
			} else if storeUser.Email != "" {
				data.Username = storeUser.Email
			} else {
				data.Username = "User"
			}

			if data.IsAdmin {
				// Add admin panel menu item (only visible to admins)
				data.NavbarItems = append([]ui.NavbarItem{{
					Label:    "Admin",
					URL:      "/admin",
					Icon:     "fa-cog",
					Position: "right",
				}}, data.NavbarItems...)
			}

			webdavURL, err := url.Parse(h.baseURL)
			if err != nil {
				slog.ErrorContext(ctx, "could not parse base url", log.Error(errors.WithStack(err)))
				return data
			}

			webdavURL.User = url.User(storeUser.BasicUsername)
			webdavURL.Path = "/dav/"

			data.WebDAVURL = webdavURL.String()
		}
	}

	fsPath := fileInfo.Name()

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

	data.BreadcrumbItems = breadcrumbs
	data.Path = fsPath
	data.Directories = dirs
	data.Files = regularFiles
	data.ParentPath = parentPath

	return data
}
