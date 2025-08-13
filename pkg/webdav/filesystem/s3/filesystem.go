package s3

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/bornholm/calli/pkg/webdav/filesystem"
	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

const (
	separator   = "/"
	keepDirFile = ".keepdir"
)

// FileSystemConfig contains configuration options for the S3 filesystem
type FileSystemConfig struct {
	// This controls the maximum concurrent uploads
	MaxFiles int
	// This controls the maximum disk space used for temporary files
	MaxTotalTempSize int64
}

// FileSystem implements the webdav.FileSystem interface for S3 storage
type FileSystem struct {
	client *minio.Client
	bucket string
	config FileSystemConfig
}

// Mkdir implements webdav.FileSystem.
func (f *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	name = clean(name)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	prefix := strings.Trim(name, separator)
	if !strings.HasSuffix(prefix, separator) {
		prefix += separator
	}

	keepDirFile := strings.Trim(filepath.Clean(prefix+keepDirFile), separator)

	if _, err := f.client.PutObject(ctx, f.bucket, keepDirFile, bytes.NewBufferString(""), -1, minio.PutObjectOptions{}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// OpenFile implements webdav.FileSystem.
func (f *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	name = clean(name)

	if flag&os.O_APPEND != 0 {
		return nil, errors.WithStack(filesystem.ErrNotSupported)
	}

	// Configure options for file uploads
	maxFiles := defaultMaxFiles
	var maxTotalTempSize int64 = defaultMaxTotalSize

	if f.config.MaxFiles > 0 {
		maxFiles = f.config.MaxFiles
	}

	if f.config.MaxTotalTempSize > 0 {
		maxTotalTempSize = f.config.MaxTotalTempSize
	}

	// Create file with temp file-based buffering
	file, err := NewFile(ctx, f.client, f.bucket, name, flag, minio.PutObjectOptions{
		ConcurrentStreamParts: true,
	}, maxFiles, maxTotalTempSize)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}

		return nil, errors.WithStack(err)
	}

	if flag&os.O_CREATE != 0 {
		if _, err := file.Write([]byte("")); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return file, nil
}

// RemoveAll implements webdav.FileSystem.
func (f *FileSystem) RemoveAll(ctx context.Context, name string) error {
	name = clean(name)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stat, err := f.Stat(ctx, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return errors.WithStack(err)
	}

	if stat.IsDir() {
		fileInfos, err := readdir(ctx, f.client, f.bucket, name, -1)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}

			return errors.WithStack(err)
		}

		for _, fi := range fileInfos {
			path := filepath.Join(name, fi.Name())

			if err := f.client.RemoveObject(ctx, f.bucket, path, minio.RemoveObjectOptions{
				ForceDelete: true,
			}); err != nil {
				return errors.WithStack(err)
			}
		}
	} else {
		if err := f.client.RemoveObject(ctx, f.bucket, name, minio.RemoveObjectOptions{
			ForceDelete: true,
		}); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// Rename implements webdav.FileSystem.
func (f *FileSystem) Rename(ctx context.Context, oldName string, newName string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	oldName = clean(oldName)
	newName = clean(newName)

	dest := minio.CopyDestOptions{
		Bucket: f.bucket,
		Object: newName,
	}

	src := minio.CopySrcOptions{
		Bucket: f.bucket,
		Object: oldName,
	}

	if _, err := f.client.CopyObject(ctx, dest, src); err != nil {
		return errors.WithStack(err)
	}

	if err := f.client.RemoveObject(ctx, f.bucket, oldName, minio.RemoveObjectOptions{
		ForceDelete: true,
	}); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Stat implements webdav.FileSystem.
func (f *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	name = clean(name)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	fileInfo, err := stat(ctx, f.client, f.bucket, name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}

		return nil, errors.WithStack(err)
	}

	return fileInfo, nil
}

// NewFileSystem creates a new S3 filesystem with the given client and bucket
func NewFileSystem(client *minio.Client, bucket string) *FileSystem {
	return &FileSystem{
		client: client,
		bucket: bucket,
		config: FileSystemConfig{},
	}
}

// NewFileSystemWithConfig creates a new S3 filesystem with custom configuration
func NewFileSystemWithConfig(client *minio.Client, bucket string, config FileSystemConfig) *FileSystem {
	return &FileSystem{
		client: client,
		bucket: bucket,
		config: config,
	}
}

var _ webdav.FileSystem = &FileSystem{}

func clean(name string) string {
	name = strings.Trim(name, separator)
	if name == "" {
		name = separator
	}
	return name
}
