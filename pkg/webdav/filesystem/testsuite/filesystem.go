package testsuite

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
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
		Run: func(ctx context.Context, fs webdav.FileSystem) error {
			if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
				return errors.WithStack(err)
			}

			path := "./Test/CreateRelativeDirectory"

			if err := fs.Mkdir(ctx, path, os.ModePerm); err != nil {
				return errors.WithStack(err)
			}

			fileInfo, err := fs.Stat(ctx, path)
			if err != nil {
				return errors.WithStack(err)
			}

			if e, g := filepath.Base(path), fileInfo.Name(); e != g {
				return errors.Errorf("fileInfo.Name: expected '%s', got '%s'", e, g)
			}

			if !fileInfo.IsDir() {
				return errors.Errorf("'%s' should be a directory", fileInfo.Name())
			}

			return nil
		},
	},
	{
		Name: "CreateAbsoluteDirectory",
		Run: func(ctx context.Context, fs webdav.FileSystem) error {
			if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
				return errors.WithStack(err)
			}

			path := "Test/CreateAbsoluteDirectory"

			if err := fs.Mkdir(ctx, path, os.ModePerm); err != nil {
				return errors.WithStack(err)
			}

			fileInfo, err := fs.Stat(ctx, path)
			if err != nil {
				return errors.WithStack(err)
			}

			if e, g := filepath.Base(path), fileInfo.Name(); e != g {
				return errors.Errorf("fileInfo.Name: expected '%s', got '%s'", e, g)
			}

			if !fileInfo.IsDir() {
				return errors.Errorf("'%s' should be a directory", fileInfo.Name())
			}

			return nil
		},
	},
	{
		Name: "WriteFile",
		Run:  WriteFile,
	},
	{
		Name: "ReadDir",
		Run: func(ctx context.Context, fs webdav.FileSystem) error {
			if err := fs.Mkdir(ctx, "Test", os.ModePerm); err != nil && !errors.Is(err, os.ErrExist) {
				return errors.WithStack(err)
			}

			dir := "Test/ReadDir"

			if err := fs.Mkdir(ctx, dir, os.ModePerm); err != nil {
				return errors.WithStack(err)
			}

			directories := []string{
				"sub1",
				"sub2",
			}

			for _, n := range directories {
				if err := fs.Mkdir(ctx, filepath.Join(dir, n), os.ModePerm); err != nil {
					return errors.WithStack(err)
				}
			}

			files := []string{
				"1.txt",
				"2.txt",
				"sub1/3.txt",
				"sub2/4.txt",
				"sub2/5.txt",
			}

			for _, n := range files {
				file, err := fs.OpenFile(ctx, filepath.Join(dir, n), os.O_CREATE, os.ModePerm)
				if err != nil {
					return errors.WithStack(err)
				}

				if err := file.Close(); err != nil {
					return errors.WithStack(err)
				}
			}

			dirFile, err := fs.OpenFile(ctx, dir, os.O_RDONLY, os.ModePerm)
			if err != nil {
				return errors.WithStack(err)
			}

			fileInfos, err := dirFile.Readdir(-1)
			if err != nil {
				return errors.WithStack(err)
			}

			if e, g := 4, len(fileInfos); e != g {
				return errors.Errorf("len(fileInfos): expected '%d', got '%d'", e, g)
			}

			return nil
		},
	},
	{
		Name: "LargeFileWrite",
		Run: func(ctx context.Context, fs webdav.FileSystem) error {
			tempDir, err := os.MkdirTemp("", "testdata-*")
			if err != nil {
				return errors.WithStack(err)
			}

			defer os.RemoveAll(tempDir)

			local, err := os.Create(filepath.Join(tempDir, "largefile"))
			if err != nil {
				return errors.WithStack(err)
			}

			if err := local.Truncate(1e8); err != nil {
				return errors.WithStack(err)
			}

			defer local.Close()

			localStat, err := local.Stat()
			if err != nil {
				return errors.WithStack(err)
			}

			localSHA, err := shasum(local)
			if err != nil {
				return errors.WithStack(err)
			}

			if _, err := local.Seek(0, io.SeekStart); err != nil {
				return errors.WithStack(err)
			}

			remote, err := fs.OpenFile(ctx, "largefile", os.O_CREATE|os.O_RDWR, os.ModePerm)
			if err != nil {
				return errors.WithStack(err)
			}

			if _, err := io.Copy(remote, local); err != nil {
				defer remote.Close()
				return errors.WithStack(err)
			}

			if err := remote.Close(); err != nil {
				return errors.WithStack(err)
			}

			remote, err = fs.OpenFile(ctx, "largefile", os.O_RDONLY, os.ModePerm)
			if err != nil {
				return errors.WithStack(err)
			}

			remoteStat, err := remote.Stat()
			if err != nil {
				return errors.WithStack(err)
			}

			if e, g := localStat.Size(), remoteStat.Size(); e != g {
				return errors.Errorf("remoteStat.Size(): expected '%d', got '%d'", e, g)
			}

			defer remote.Close()

			remoteSHA, err := shasum(remote)
			if err != nil {
				return errors.WithStack(err)
			}

			if e, g := localSHA, remoteSHA; e != g {
				return errors.Errorf("sha256: expected '%s', got '%s'", e, g)
			}

			return nil
		},
	},
}

func shasum(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", errors.WithStack(err)
	}

	hash := sha256.Sum256(data)

	return fmt.Sprintf("%x", hash), nil
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
