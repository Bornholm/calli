package local

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bornholm/calli/pkg/webdav/filesystem/testsuite"
	"github.com/pkg/errors"
)

func TestFileSystem(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	dir := filepath.Join(cwd, "testdata/.local")

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	testsuite.TestFileSystem(t, Type, &Options{
		Dir: dir,
	})
}
