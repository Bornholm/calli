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

// Default buffer size for each chunk in the buffer pool
const (
	defaultBufferSize  = 1 << 20 // 1 MB per buffer
	defaultBufferCount = 16      // Max 16 MB of memory for upload buffers
)

// bufferPool manages a pool of byte slices for file uploads
var bufferPool = sync.Pool{
	New: func() interface{} {
		buffer := make([]byte, defaultBufferSize)
		return &buffer
	},
}

// memoryBuffer implements a memory-capped buffer that reuses pooled buffers
type memoryBuffer struct {
	ctx       context.Context
	cancel    context.CancelFunc
	client    *minio.Client
	bucket    string
	key       string
	opts      minio.PutObjectOptions
	buffers   chan *[]byte // Channel of available buffers
	ready     chan *[]byte // Buffers ready to be uploaded
	wg        sync.WaitGroup
	uploading sync.WaitGroup
	err       error
	mu        sync.Mutex // Protects err
}

// newMemoryBuffer creates a new memory buffer with controlled memory usage
func newMemoryBuffer(ctx context.Context, client *minio.Client, bucket, key string, opts minio.PutObjectOptions, bufferSize, bufferCount int) *memoryBuffer {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}

	if bufferCount <= 0 {
		bufferCount = defaultBufferCount
	}

	ctx, cancel := context.WithCancel(ctx)

	mb := &memoryBuffer{
		ctx:     ctx,
		cancel:  cancel,
		client:  client,
		bucket:  bucket,
		key:     key,
		opts:    opts,
		buffers: make(chan *[]byte, bufferCount),
		ready:   make(chan *[]byte, bufferCount),
	}

	// Initialize buffer pool with custom-sized buffers
	for i := 0; i < bufferCount; i++ {
		buffer := make([]byte, bufferSize)
		mb.buffers <- &buffer
	}

	// Start the uploader goroutine
	mb.wg.Add(1)
	go mb.uploader()

	return mb
}

// uploader processes buffers and uploads them to S3
func (mb *memoryBuffer) uploader() {
	defer mb.wg.Done()

	// Create a pipe for streaming to S3
	pr, pw := io.Pipe()
	mb.uploading.Add(1)

	// Start the S3 upload
	go func() {
		defer mb.uploading.Done()
		// size = -1 for unknown / streaming
		_, err := mb.client.PutObject(mb.ctx, mb.bucket, mb.key, pr, -1, mb.opts)
		if err != nil {
			mb.mu.Lock()
			mb.err = errors.WithStack(err)
			mb.mu.Unlock()
			pr.CloseWithError(errors.WithStack(err))
		}
	}()

	// Process buffers
	for {
		select {
		case <-mb.ctx.Done():
			pw.CloseWithError(mb.ctx.Err())
			return
		case buffer, ok := <-mb.ready:
			if !ok {
				// Channel closed, we're done
				pw.Close()
				return
			}
			// Write buffer to pipe
			_, err := pw.Write(*buffer)
			if err != nil {
				mb.mu.Lock()
				mb.err = errors.WithStack(err)
				mb.mu.Unlock()
				pw.CloseWithError(err)
				return
			}
			// Return buffer to pool
			mb.buffers <- buffer
		}
	}
}

// Write implements io.Writer interface
func (mb *memoryBuffer) Write(p []byte) (n int, err error) {
	totalWritten := 0
	for totalWritten < len(p) {
		// Check for error or cancellation
		select {
		case <-mb.ctx.Done():
			return totalWritten, mb.ctx.Err()
		default:
		}

		mb.mu.Lock()
		if mb.err != nil {
			err := mb.err
			mb.mu.Unlock()
			return totalWritten, err
		}
		mb.mu.Unlock()

		// Get buffer from pool - will block if all buffers are in use
		var buffer *[]byte
		select {
		case buffer = <-mb.buffers:
			// Got a buffer
		case <-mb.ctx.Done():
			return totalWritten, mb.ctx.Err()
		}

		// Copy data to buffer
		bytesToCopy := len(p) - totalWritten
		if bytesToCopy > len(*buffer) {
			bytesToCopy = len(*buffer)
		}
		copy(*buffer, p[totalWritten:totalWritten+bytesToCopy])

		// Slice the buffer to actual data size
		actualBuffer := (*buffer)[:bytesToCopy]
		*buffer = actualBuffer

		// Send buffer for upload
		select {
		case mb.ready <- buffer:
			// Buffer sent for processing
		case <-mb.ctx.Done():
			// Return buffer to pool
			mb.buffers <- buffer
			return totalWritten, mb.ctx.Err()
		}

		totalWritten += bytesToCopy
	}

	return totalWritten, nil
}

// Close flushes all buffers and waits for upload to complete
func (mb *memoryBuffer) Close() error {
	// Signal that no more data is coming
	close(mb.ready)

	// Wait for uploader to finish
	mb.wg.Wait()

	// Wait for S3 upload to finish
	mb.uploading.Wait()

	// Check for errors
	mb.mu.Lock()
	err := mb.err
	mb.mu.Unlock()

	// Clean up resources
	mb.cancel()

	// Release all buffers
	close(mb.buffers)
	for buffer := range mb.buffers {
		bufferPool.Put(buffer)
	}

	return err
}

// File represents a file in the S3 filesystem
type File struct {
	ctx    context.Context
	cancel context.CancelFunc

	client *minio.Client
	bucket string
	key    string

	// For reads
	obj *minio.Object

	// For writes
	memBuf *memoryBuffer
	wg     sync.WaitGroup
}

// Close implements webdav.File.
func (f *File) Close() error {
	defer f.cancel()

	var errR, errW error

	if f.obj != nil {
		errR = f.obj.Close()
	}

	if f.memBuf != nil {
		errW = f.memBuf.Close()
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
	if f.memBuf != nil {
		err := f.memBuf.Close()
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
	if f.memBuf == nil {
		return 0, os.ErrClosed
	}

	return f.memBuf.Write(p)
}

// NewFile creates a new S3 file with memory-capped upload capability
func NewFile(ctx context.Context, client *minio.Client, bucket, key string, flag int, opts minio.PutObjectOptions, bufferSize, bufferCount int) (*File, error) {
	f := &File{client: client, bucket: bucket, key: key}

	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.ctx = ctx

	write := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0
	read := flag == 0 || flag&os.O_RDWR != 0

	if write {
		// Set up a memory-capped buffer for uploads with custom buffer settings
		f.memBuf = newMemoryBuffer(ctx, client, bucket, key, opts, bufferSize, bufferCount)
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
