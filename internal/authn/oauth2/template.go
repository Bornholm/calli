package oauth2

import (
	"embed"
	"html/template"

	"github.com/bornholm/calli/internal/ui"
	"github.com/pkg/errors"
)

//go:embed templates/**/*.gohtml
var fs embed.FS

var templates *template.Template

func init() {
	t, err := ui.Templates(nil, fs)
	if err != nil {
		panic(errors.WithStack(err))
	}
	templates = t
}
