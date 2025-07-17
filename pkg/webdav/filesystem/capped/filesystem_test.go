package capped

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bornholm/calli/pkg/webdav/filesystem/local"
	"github.com/bornholm/calli/pkg/webdav/filesystem/testsuite"
	"github.com/pkg/errors"
)

func TestFileSystem(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	dataDir := filepath.Join(cwd, "testdata/.local")

	if err := os.RemoveAll(dataDir); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	if err := os.MkdirAll(dataDir, os.ModePerm); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	testsuite.TestFileSystem(t, Type, &Options{
		MaxSize: 1e3,
		Backend: FileSystemOptions{
			Type: local.Type,
			Options: local.Options{
				Dir: dataDir,
			},
		},
	})
}
