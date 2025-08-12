package explorer

import (
	"embed"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/bornholm/calli/internal/ui"
	"github.com/dustin/go-humanize"
	"github.com/pkg/errors"
)

//go:embed templates/**
var templateFs embed.FS

var templates *template.Template

func init() {
	tmpl, err := ui.Templates(nil, templateFs)
	if err != nil {
		panic(errors.WithStack(err))
	}

	templates = tmpl
}

// FileTemplateData contains information about a file to be displayed in the file explorer
type FileTemplateData struct {
	Name      string
	Path      string
	Size      int64
	HumanSize string
	ModTime   time.Time
	IsDir     bool
	IsImage   bool
	IsText    bool
	IsArchive bool
	IsAudio   bool
	IsVideo   bool
	IsCode    bool
	IsPDF     bool
}

// FileExplorerTemplateData contains the data needed to render the file explorer view
type FileExplorerTemplateData struct {
	ui.HeadTemplateData
	ui.NavbarTemplateData
	Path            string
	ParentPath      string
	BreadcrumbItems []string
	Directories     []FileTemplateData
	Files           []FileTemplateData
	IsAdmin         bool
	Username        string
	WebDAVURL       string
	FlashMessage    string
}

// NewFileTemplateData creates a new file data structure from an os.FileInfo
func NewFileTemplateData(info os.FileInfo, currentPath string) FileTemplateData {
	name := info.Name()
	filePath := path.Join(currentPath, name)
	fileData := FileTemplateData{
		Name:      name,
		Path:      filePath,
		Size:      info.Size(),
		HumanSize: humanize.Bytes(uint64(info.Size())),
		ModTime:   info.ModTime(),
		IsDir:     info.IsDir(),
	}

	if !info.IsDir() {
		ext := strings.ToLower(filepath.Ext(name))

		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg":
			fileData.IsImage = true
		case ".txt", ".md", ".csv", ".json", ".xml", ".yaml", ".yml":
			fileData.IsText = true
		case ".zip", ".tar", ".gz", ".bz2", ".rar", ".7z":
			fileData.IsArchive = true
		case ".mp3", ".wav", ".ogg", ".flac", ".aac":
			fileData.IsAudio = true
		case ".mp4", ".avi", ".mov", ".wmv", ".mkv", ".webm":
			fileData.IsVideo = true
		case ".go", ".js", ".ts", ".html", ".css", ".py", ".java", ".c", ".cpp", ".php", ".rb":
			fileData.IsCode = true
		case ".pdf":
			fileData.IsPDF = true
		}
	}

	return fileData
}
