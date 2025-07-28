package ui

import (
	"embed"
	"html/template"
	"io/fs"

	"github.com/Masterminds/sprig/v3"
	"github.com/dustin/go-humanize"
	"github.com/laher/mergefs"
	"github.com/pkg/errors"
)

//go:embed templates/**
var commonFs embed.FS

var templates *template.Template

var commonFuncs = template.FuncMap{
	"humanizeInt": func(n int) string {
		return humanize.Comma(int64(n))
	},
	"subtract": func(a, b int) int {
		return a - b
	},
}

func Templates(funcs template.FuncMap, filesystems ...fs.FS) (*template.Template, error) {
	filesystems = append([]fs.FS{commonFs}, filesystems...)
	merged := mergefs.Merge(filesystems...)

	views, err := fs.Glob(merged, "**/views/*.gohtml")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	layouts, err := fs.Glob(merged, "**/layouts/*.gohtml")
	if err != nil {
		return nil, errors.WithStack(err)
	}

	templates := append(views, layouts...)

	tmpl := template.New("").Funcs(sprig.FuncMap()).Funcs(commonFuncs)

	if funcs != nil {
		tmpl = tmpl.Funcs(funcs)
	}

	tmpl, err = tmpl.ParseFS(merged, templates...)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return tmpl, nil
}

type HeadTemplateData struct {
	PageTitle string
}
