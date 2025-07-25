package s3

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
)

const (
	defaultFileMode = 0o644
)

func readdir(ctx context.Context, client *minio.Client, bucket string, name string, count int, ignored ...string) ([]os.FileInfo, error) {
	prefix := clean(name)

	if prefix == "." || prefix == separator {
		prefix = ""
	} else {
		prefix = strings.Trim(prefix, separator) + separator
	}

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	}

	ch := client.ListObjects(ctx, bucket, opts)
	var fis []os.FileInfo
	for obj := range ch {
		if obj.Err != nil {
			return fis, errors.WithStack(obj.Err)
		}

		// skip the directory itself
		if strings.TrimSuffix(obj.Key, separator) == strings.TrimPrefix(name, separator) {
			continue
		}

		if len(ignored) > 0 && slices.Index(ignored, filepath.Base(obj.Key)) != -1 {
			continue
		}

		fis = append(fis, FromObjectInfo(obj))

		if count > 0 && len(fis) >= count {
			return fis, nil
		}
	}

	if count > 0 && len(fis) == 0 {
		return fis, io.EOF
	}

	return fis, nil
}

func stat(ctx context.Context, client *minio.Client, bucket string, name string) (os.FileInfo, error) {
	name = clean(name)

	if name == "." || name == separator {
		return &FileInfo{
			isDir:   true,
			modTime: time.Now(),
			mode:    defaultFileMode,
			name:    filepath.Base(name),
			size:    4096,
		}, nil
	}

	name = filepath.Clean(name)

	stat, err := client.StatObject(ctx, bucket, strings.TrimPrefix(name, separator), minio.GetObjectOptions{})
	if err != nil {
		errRes := minio.ToErrorResponse(err)
		if errRes.Code == "NoSuchKey" {
			fileInfo, err := statDir(ctx, client, bucket, name)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, os.ErrNotExist
				}

				return nil, errors.WithStack(err)
			}

			return fileInfo, nil
		}

		return nil, errors.WithStack(err)
	}

	return &FileInfo{
		isDir:   false,
		modTime: stat.LastModified,
		mode:    defaultFileMode,
		name:    filepath.Base(name),
		size:    stat.Size,
	}, nil
}

func statDir(ctx context.Context, client *minio.Client, bucket string, name string) (os.FileInfo, error) {
	name = clean(name)
	prefix := name + separator

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	opts := minio.ListObjectsOptions{
		Prefix: prefix,
	}

	objects := client.ListObjects(ctx, bucket, opts)

	fileInfo := &FileInfo{
		isDir:   true,
		modTime: time.Time{},
		mode:    os.ModeDir | defaultFileMode,
		name:    filepath.Base(name),
		size:    4096,
	}

	for obj := range objects {
		if obj.Err != nil {
			return nil, errors.WithStack(obj.Err)
		}

		if obj.LastModified.After(fileInfo.ModTime()) {
			fileInfo.modTime = obj.LastModified
		}
	}

	if !fileInfo.modTime.IsZero() {
		return fileInfo, nil
	}

	return nil, os.ErrNotExist
}
