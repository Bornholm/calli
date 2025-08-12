package testsuite

import (
	"context"
	"log/slog"
	"testing"
	"time"

	wd "github.com/bornholm/calli/pkg/webdav"
	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

type filesystemTestCase struct {
	Name string
	Run  func(ctx context.Context, fs webdav.FileSystem) error
}

var filesystemTestCases = []filesystemTestCase{
	{
		Name: "CreateRelativeDirectory",
		Run:  CreateRelativeDirectory,
	},
	{
		Name: "CreateAbsoluteDirectory",
		Run:  CreateAbsoluteDirectory,
	},
	{
		Name: "WriteFile",
		Run:  WriteFile,
	},
	{
		Name: "ReadDir",
		Run:  ReadDir,
	},
	{
		Name: "LargeFileWrite",
		Run:  LargeFileWrite,
	},
	{
		Name: "DeleteFile",
		Run:  DeleteFile,
	},
	{
		Name: "ModifyFile",
		Run:  ModifyFile,
	},
	{
		Name: "FileMetadata",
		Run:  FileMetadata,
	},
	{
		Name: "RecursiveDirectory",
		Run:  RecursiveDirectory,
	},
}

func TestFileSystem(t *testing.T, fsType filesystem.Type, opts any) {
	t.Logf("Using filesystem '%s'", fsType)

	fs, err := filesystem.New(fsType, opts)
	if err != nil {
		t.Fatalf("%+v", errors.WithStack(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fs = wd.WithLogger(fs, slog.Default())

	for _, tc := range filesystemTestCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			if err := tc.Run(ctx, fs); err != nil {
				t.Errorf("%+v", errors.WithStack(err))
			}
		})
	}
}
