package sqlite

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

	dbPath := filepath.Join(cwd, "testdata", "webdav.db")

	if err := os.RemoveAll(dbPath); err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	// Run the standard filesystem test suite with our SQLite implementation
	testsuite.TestFileSystem(t, Type, &Options{
		Path: dbPath,
	})
}
