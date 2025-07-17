package s3

import (
	"context"
	"io"
	"io/fs"
	"os"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

type File struct {
	ctx    context.Context
	cancel context.CancelFunc

	client *minio.Client
	bucket string
	key    string

	// For reads
	obj *minio.Object

	// For writes
	pr *io.PipeReader
	pw *io.PipeWriter
	wg sync.WaitGroup
}

// Close implements webdav.File.
func (f *File) Close() error {
	defer f.cancel()

	var errR, errW error

	if f.obj != nil {
		errR = f.obj.Close()
	}

	if f.pw != nil {
		// closing writer signals EOF to PutObject
		errW = f.pw.Close()
		f.wg.Wait()
	}

	if errR != nil {
		return errors.WithStack(errR)
	}

	return errors.WithStack(errW)
}

// Read implements webdav.File.
func (f *File) Read(p []byte) (n int, err error) {
	if f.obj == nil {
		return 0, os.ErrClosed
	}

	return f.obj.Read(p)
}

// Readdir implements webdav.File.
func (f *File) Readdir(count int) ([]fs.FileInfo, error) {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	return readdir(ctx, f.client, f.bucket, f.key, count, keepDirFile)
}

// Seek implements webdav.File.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.obj == nil {
		return 0, os.ErrClosed
	}

	return f.obj.Seek(offset, whence)
}

// Stat implements webdav.File.
func (f *File) Stat() (fs.FileInfo, error) {
	if f.pw != nil {
		err := f.pw.Close()
		if err != nil {
			return nil, errors.WithStack(err)
		}

		f.wg.Wait()
	}

	info, err := stat(f.ctx, f.client, f.bucket, f.key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, os.ErrNotExist
		}

		return nil, errors.WithStack(err)
	}

	return info, nil
}

// Write implements webdav.File.
func (f *File) Write(p []byte) (n int, err error) {
	if f.pw == nil {
		return 0, os.ErrClosed
	}

	return f.pw.Write(p)
}

func NewFile(ctx context.Context, client *minio.Client, bucket, key string, flag int, opts minio.PutObjectOptions) (*File, error) {
	f := &File{client: client, bucket: bucket, key: key}

	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.ctx = ctx

	write := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0
	read := flag == 0 || flag&os.O_RDWR != 0

	if write {
		// set up a pipe: Write -> pw, background goroutine reads pr -> PutObject
		pr, pw := io.Pipe()
		f.pr, f.pw = pr, pw
		f.wg.Add(1)

		go func() {
			defer f.wg.Done()
			// size = -1 for unknown / streaming
			_, err := client.PutObject(ctx, bucket, key, pr, -1, opts)
			// if PutObject failed, propagate to reader
			pr.CloseWithError(errors.WithStack(err))
		}()

		return f, nil
	}

	if read {
		// open for read
		obj, err := client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
		if err != nil {
			return nil, errors.WithStack(err)
		}
		f.obj = obj
		return f, nil
	}

	return nil, errors.New("must open for read or write")
}

var _ webdav.File = &File{}
