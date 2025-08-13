package s3

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// Default settings for the streaming buffer
const (
	defaultBufferSize   = 10 * 1024 * 1024 // 10 MB per buffer
	defaultMaxParts     = 10000            // Maximum number of parts (S3 limit)
	defaultMaxFiles     = 16               // Maximum number of concurrent files
	defaultMaxTotalSize = 1 << 30          // 1 GB maximum total storage (unused in streaming implementation)
	defaultPartPrefix   = ".parts"         // Prefix for part objects
)

// streamingBuffer implements a buffer that streams directly to S3
type streamingBuffer struct {
	ctx    context.Context
	cancel context.CancelFunc
	client *minio.Client
	bucket string
	key    string
	opts   minio.PutObjectOptions

	buffer     []byte      // In-memory buffer for current part
	bufferPos  int         // Current position in buffer
	partNum    int         // Current part number
	partKeys   []string    // Keys of uploaded parts
	partPrefix string      // Prefix for part keys
	totalSize  int64       // Total size of all parts
	err        error       // Any error that occurred
	mu         sync.Mutex  // Protects state
	closed     atomic.Bool // Indicates if the buffer is closed
}

// newStreamingBuffer creates a new buffer that streams directly to S3
func newStreamingBuffer(ctx context.Context, client *minio.Client, bucket, key string, opts minio.PutObjectOptions, bufferSize int) (*streamingBuffer, error) {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}

	ctx, cancel := context.WithCancel(ctx)

	sb := &streamingBuffer{
		ctx:        ctx,
		cancel:     cancel,
		client:     client,
		bucket:     bucket,
		key:        key,
		opts:       opts,
		buffer:     make([]byte, bufferSize),
		partPrefix: fmt.Sprintf("%s/%s", defaultPartPrefix, key),
		partKeys:   make([]string, 0, defaultMaxParts),
	}

	return sb, nil
}

// Write implements io.Writer interface
func (sb *streamingBuffer) Write(p []byte) (n int, err error) {
	if sb.closed.Load() {
		return 0, os.ErrClosed
	}

	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.err != nil {
		return 0, sb.err
	}

	totalWritten := 0
	remaining := len(p)

	for remaining > 0 {
		// Calculate space left in the current buffer
		spaceLeft := len(sb.buffer) - sb.bufferPos

		// If no space left, flush the buffer
		if spaceLeft == 0 {
			if err := sb.flushBufferLocked(); err != nil {
				sb.err = err
				return totalWritten, err
			}
			spaceLeft = len(sb.buffer)
		}

		// Calculate how much to copy in this iteration
		toCopy := remaining
		if toCopy > spaceLeft {
			toCopy = spaceLeft
		}

		// Copy data into the buffer
		copy(sb.buffer[sb.bufferPos:], p[totalWritten:totalWritten+toCopy])
		sb.bufferPos += toCopy
		totalWritten += toCopy
		remaining -= toCopy
	}

	return totalWritten, nil
}

// flushBufferLocked uploads the current buffer as a part
// Caller must hold the lock
func (sb *streamingBuffer) flushBufferLocked() error {
	if sb.bufferPos == 0 {
		return nil // Nothing to flush
	}

	// Create a reader for the current buffer
	partData := bytes.NewReader(sb.buffer[:sb.bufferPos])
	partSize := int64(sb.bufferPos)

	// Generate a unique key for this part
	partKey := fmt.Sprintf("%s/%d", sb.partPrefix, sb.partNum)

	// Upload the part
	_, err := sb.client.PutObject(
		sb.ctx,
		sb.bucket,
		partKey,
		partData,
		partSize,
		sb.opts,
	)
	if err != nil {
		return errors.WithStack(err)
	}

	// Add this part to our list
	sb.partKeys = append(sb.partKeys, partKey)
	sb.totalSize += partSize
	sb.partNum++
	sb.bufferPos = 0

	return nil
}

// Close finalizes the upload by composing all parts and cleaning up
func (sb *streamingBuffer) Close() error {
	// Use atomic swap to ensure we only process close once
	if sb.closed.Swap(true) {
		return nil // Already closed
	}

	defer sb.cancel()

	sb.mu.Lock()
	defer sb.mu.Unlock()

	// Check for existing errors
	if sb.err != nil {
		// Clean up any uploaded parts before returning
		sb.cleanupPartsLocked()
		return sb.err
	}

	// Flush any remaining data in the buffer
	if sb.bufferPos > 0 {
		if err := sb.flushBufferLocked(); err != nil {
			sb.err = err
			sb.cleanupPartsLocked()
			return err
		}
	}

	// If we have no parts, create an empty object
	if len(sb.partKeys) == 0 {
		_, err := sb.client.PutObject(
			sb.ctx,
			sb.bucket,
			sb.key,
			bytes.NewReader([]byte{}),
			0,
			sb.opts,
		)
		if err != nil {
			sb.err = errors.WithStack(err)
			return sb.err
		}
		return nil
	}

	// If we only have one part, just copy it to the final destination
	if len(sb.partKeys) == 1 {
		src := minio.CopySrcOptions{
			Bucket: sb.bucket,
			Object: sb.partKeys[0],
		}
		dst := minio.CopyDestOptions{
			Bucket: sb.bucket,
			Object: sb.key,
		}

		_, err := sb.client.CopyObject(sb.ctx, dst, src)
		if err != nil {
			sb.err = errors.WithStack(err)
			sb.cleanupPartsLocked()
			return sb.err
		}
	} else {
		// If we have multiple parts, we need to manually concatenate them
		// by reading each part and concatenating to a final object
		// (since ComposeObject doesn't exist in minio-go/v7)

		// Create a temporary object that will hold the combined data
		var currentSize int64

		// Start with first part
		src := minio.CopySrcOptions{
			Bucket: sb.bucket,
			Object: sb.partKeys[0],
		}
		dst := minio.CopyDestOptions{
			Bucket: sb.bucket,
			Object: sb.key,
		}

		_, err := sb.client.CopyObject(sb.ctx, dst, src)
		if err != nil {
			sb.err = errors.WithStack(err)
			sb.cleanupPartsLocked()
			return sb.err
		}

		// For each subsequent part, append to the object
		// Note: This is inefficient for large numbers of parts or large parts
		// but without ComposeObject, this is a functional approach
		for i := 1; i < len(sb.partKeys); i++ {
			// Get part data
			partObj, err := sb.client.GetObject(sb.ctx, sb.bucket, sb.partKeys[i], minio.GetObjectOptions{})
			if err != nil {
				sb.err = errors.WithStack(err)
				sb.cleanupPartsLocked()
				return sb.err
			}

			// Get original object
			destObj, err := sb.client.GetObject(sb.ctx, sb.bucket, sb.key, minio.GetObjectOptions{})
			if err != nil {
				partObj.Close()
				sb.err = errors.WithStack(err)
				sb.cleanupPartsLocked()
				return sb.err
			}

			// Create a buffer that combines both
			var combined bytes.Buffer

			// Copy current object to buffer
			_, err = combined.ReadFrom(destObj)
			destObj.Close()
			if err != nil {
				partObj.Close()
				sb.err = errors.WithStack(err)
				sb.cleanupPartsLocked()
				return sb.err
			}

			// Add part data
			_, err = combined.ReadFrom(partObj)
			partObj.Close()
			if err != nil {
				sb.err = errors.WithStack(err)
				sb.cleanupPartsLocked()
				return sb.err
			}

			// Update combined size
			currentSize = int64(combined.Len())

			// Put back the combined object
			_, err = sb.client.PutObject(
				sb.ctx,
				sb.bucket,
				sb.key,
				&combined,
				currentSize,
				sb.opts,
			)
			if err != nil {
				sb.err = errors.WithStack(err)
				sb.cleanupPartsLocked()
				return sb.err
			}
		}
	}

	// Clean up the part objects
	return sb.cleanupPartsLocked()
}

// cleanupPartsLocked removes all uploaded part objects
// Caller must hold the lock
func (sb *streamingBuffer) cleanupPartsLocked() error {
	if len(sb.partKeys) == 0 {
		return nil
	}

	var firstErr error

	// Remove each part individually
	for _, partKey := range sb.partKeys {
		err := sb.client.RemoveObject(sb.ctx, sb.bucket, partKey, minio.RemoveObjectOptions{
			ForceDelete: true,
		})
		if err != nil && firstErr == nil {
			firstErr = errors.Wrapf(err, "failed to remove part object: %s", partKey)
		}
	}

	// Clear the parts list regardless of errors
	sb.partKeys = sb.partKeys[:0]

	return firstErr
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
	streamBuf *streamingBuffer
	wg        sync.WaitGroup
}

// Close implements webdav.File.
func (f *File) Close() error {
	defer f.cancel()

	var errR, errW error

	// Safely close the read object if present
	if f.obj != nil {
		obj := f.obj
		f.obj = nil // Clear reference to prevent double-close
		errR = obj.Close()
	}

	// Safely close the streaming buffer if present
	if f.streamBuf != nil {
		streamBuf := f.streamBuf
		f.streamBuf = nil // Clear reference to prevent double-close
		errW = streamBuf.Close()
		f.wg.Wait()
	}

	if errR != nil {
		return errors.WithStack(errR)
	}

	// Handle the write error
	if errW != nil {
		// Ignore file already closed errors
		if errors.Is(errW, os.ErrClosed) || strings.Contains(errW.Error(), "file already closed") {
			return nil
		}
		return errors.WithStack(errW)
	}

	return nil
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

	return readdir(ctx, f.client, f.bucket, f.key, count, keepDirFile, defaultPartPrefix)
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
	if f.streamBuf != nil {
		err := f.streamBuf.Close()
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
	if f.streamBuf == nil {
		return 0, os.ErrClosed
	}

	return f.streamBuf.Write(p)
}

// NewFile creates a new S3 file with streaming upload
func NewFile(ctx context.Context, client *minio.Client, bucket, key string, flag int, opts minio.PutObjectOptions, maxFiles int, maxTotalTempSize int64) (*File, error) {
	f := &File{client: client, bucket: bucket, key: key}

	ctx, cancel := context.WithCancel(ctx)
	f.cancel = cancel
	f.ctx = ctx

	write := flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC) != 0
	read := flag == 0 || flag&os.O_RDWR != 0

	if write {
		// Calculate buffer size (use 10MB as default)
		bufferSize := defaultBufferSize

		// Set up a streaming buffer for uploads
		streamBuf, err := newStreamingBuffer(ctx, client, bucket, key, opts, bufferSize)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		f.streamBuf = streamBuf
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
