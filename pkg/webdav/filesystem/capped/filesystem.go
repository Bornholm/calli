package capped

import (
	"context"
	"io"
	"os"
	"path"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/webdav"
)

// FileSystem implements webdav.FileSystem with a size cap
// When the total size of files exceeds maxSize, the least recently accessed files
// are deleted to maintain the size limit.
type FileSystem struct {
	fs      webdav.FileSystem
	maxSize int64

	mu      sync.RWMutex
	files   map[string]*fileInfo
	curSize int64

	// Flag to indicate if initial scan has been done
	initialized bool
}

// fileInfo tracks metadata about files for size management and cleanup
type fileInfo struct {
	size       int64
	lastAccess time.Time
	path       string
	isDir      bool
}

// File wraps a webdav.File to update access times on reads and writes
type File struct {
	ctx  context.Context
	file webdav.File
	fs   *FileSystem
	path string
}

// Mkdir implements webdav.FileSystem.
func (f *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	// Ensure the filesystem is initialized
	if err := f.ensureInitialized(ctx); err != nil {
		return err
	}

	err := f.fs.Mkdir(ctx, name, perm)
	if err != nil {
		return err
	}

	// Add directory to tracking
	f.mu.Lock()
	defer f.mu.Unlock()
	f.files[name] = &fileInfo{
		size:       0, // Directories don't count toward size
		lastAccess: time.Now(),
		path:       name,
		isDir:      true,
	}

	return nil
}

// OpenFile implements webdav.FileSystem.
func (f *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	// Ensure the filesystem is initialized
	if err := f.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	// Check if this is a write operation
	isWriting := flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0

	// For write operations, make space if needed before allowing the write
	if isWriting && flag&os.O_CREATE != 0 {
		// If we're creating a new file, ensure we have space
		// We don't know the size yet, but ensure cleanup if we're near limit
		if err := f.ensureSpace(ctx, name, 0); err != nil {
			return nil, errors.Wrap(err, "failed to ensure space for file creation")
		}
	} else {
		// For read operations, update the access time
		f.updateAccessTime(name)
	}

	file, err := f.fs.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	return &File{
		file: file,
		fs:   f,
		path: name,
	}, nil
}

// RemoveAll implements webdav.FileSystem.
func (f *FileSystem) RemoveAll(ctx context.Context, name string) error {
	// Ensure the filesystem is initialized
	if err := f.ensureInitialized(ctx); err != nil {
		return err
	}

	// First collect info about what we're removing
	info, err := f.fs.Stat(ctx, name)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove from the underlying filesystem
	err = f.fs.RemoveAll(ctx, name)
	if err != nil {
		return err
	}

	// Update our size tracking
	f.mu.Lock()
	defer f.mu.Unlock()

	// If it's a directory, we need to find and remove all contained files from tracking
	if err == nil && info.IsDir() {
		prefix := name
		if !os.IsPathSeparator(prefix[len(prefix)-1]) {
			prefix = prefix + "/"
		}

		// Remove all files that start with this directory prefix
		for path, fileInfo := range f.files {
			if path == name || (len(path) > len(prefix) && path[:len(prefix)] == prefix) {
				if !fileInfo.isDir {
					f.curSize -= fileInfo.size
				}
				delete(f.files, path)
			}
		}
	} else {
		// Just remove the single file
		fileInfo, exists := f.files[name]
		if exists {
			if !fileInfo.isDir {
				f.curSize -= fileInfo.size
			}
			delete(f.files, name)
		}
	}

	return nil
}

// Rename implements webdav.FileSystem.
func (f *FileSystem) Rename(ctx context.Context, oldName string, newName string) error {
	// Ensure the filesystem is initialized
	if err := f.ensureInitialized(ctx); err != nil {
		return err
	}

	// Get info about the file before renaming
	oldInfo, err := f.fs.Stat(ctx, oldName)
	if err != nil {
		return err
	}
	isDir := oldInfo.IsDir()

	// Perform the rename on the underlying filesystem
	err = f.fs.Rename(ctx, oldName, newName)
	if err != nil {
		return err
	}

	// Update our tracking maps
	f.mu.Lock()
	defer f.mu.Unlock()

	if isDir {
		// If it's a directory, we need to update all contained files
		oldPrefix := oldName
		if !os.IsPathSeparator(oldPrefix[len(oldPrefix)-1]) {
			oldPrefix = oldPrefix + "/"
		}

		newPrefix := newName
		if !os.IsPathSeparator(newPrefix[len(newPrefix)-1]) {
			newPrefix = newPrefix + "/"
		}

		// Update paths of all contained files
		for path, fi := range f.files {
			if path == oldName {
				// The directory itself
				newFileInfo := &fileInfo{
					size:       fi.size,
					lastAccess: fi.lastAccess,
					path:       newName,
					isDir:      true,
				}
				f.files[newName] = newFileInfo
				delete(f.files, oldName)
			} else if len(path) > len(oldPrefix) && path[:len(oldPrefix)] == oldPrefix {
				// A file inside the directory
				newPath := newPrefix + path[len(oldPrefix):]
				newFileInfo := &fileInfo{
					size:       fi.size,
					lastAccess: fi.lastAccess,
					path:       newPath,
					isDir:      fi.isDir,
				}
				f.files[newPath] = newFileInfo
				delete(f.files, path)
			}
		}
	} else {
		// Just a regular file
		fi, exists := f.files[oldName]
		if exists {
			// Create entry with new name
			f.files[newName] = &fileInfo{
				size:       fi.size,
				lastAccess: fi.lastAccess,
				path:       newName,
				isDir:      false,
			}
			// Remove old entry
			delete(f.files, oldName)
		}
	}

	return nil
}

// Stat implements webdav.FileSystem.
func (f *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	// Ensure the filesystem is initialized
	if err := f.ensureInitialized(ctx); err != nil {
		return nil, err
	}

	info, err := f.fs.Stat(ctx, name)
	if err != nil {
		return nil, err
	}

	// Update tracking for this file
	f.mu.Lock()
	defer f.mu.Unlock()

	fi, exists := f.files[name]
	isDir := info.IsDir()

	if exists {
		// Update access time
		fi.lastAccess = time.Now()

		// Update size if it's a file (not a directory)
		if !isDir {
			// If size changed, update our tracking
			if fi.size != info.Size() {
				f.curSize = f.curSize - fi.size + info.Size()
				fi.size = info.Size()
			}
		}
	} else {
		// Add to tracking
		var fileSize int64
		if isDir {
			fileSize = 0
		} else {
			fileSize = info.Size()
		}

		f.files[name] = &fileInfo{
			size:       fileSize,
			lastAccess: time.Now(),
			path:       name,
			isDir:      isDir,
		}

		if !isDir {
			f.curSize += info.Size()
		}
	}

	return info, nil
}

// ensureInitialized makes sure the filesystem has been scanned for initial size calculation
func (f *FileSystem) ensureInitialized(ctx context.Context) error {
	f.mu.Lock()
	if f.initialized {
		f.mu.Unlock()
		return nil
	}
	f.initialized = true
	f.mu.Unlock()

	// Do the initial scan outside the lock to avoid holding the lock for too long
	return f.scanDirectory(ctx, "/")
}

// scanDirectory recursively scans a directory to build initial size tracking
func (f *FileSystem) scanDirectory(ctx context.Context, dirPath string) error {
	// Open the directory
	dir, err := f.fs.OpenFile(ctx, dirPath, os.O_RDONLY, 0)
	if err != nil {
		return errors.Wrapf(err, "failed to open directory %s for scanning", dirPath)
	}
	defer dir.Close()

	// Read all entries
	entries, err := dir.Readdir(-1)
	if err != nil {
		return errors.Wrapf(err, "failed to read directory %s", dirPath)
	}

	// Process each entry
	for _, entry := range entries {
		fullPath := path.Join(dirPath, entry.Name())

		// Add to tracking
		f.mu.Lock()
		if entry.IsDir() {
			f.files[fullPath] = &fileInfo{
				size:       0, // Directories don't count toward size
				lastAccess: time.Now(),
				path:       fullPath,
				isDir:      true,
			}
			f.mu.Unlock()

			// Recursively scan subdirectories
			if err := f.scanDirectory(ctx, fullPath); err != nil {
				return err
			}
		} else {
			// Regular file
			size := entry.Size()
			f.files[fullPath] = &fileInfo{
				size:       size,
				lastAccess: time.Now(),
				path:       fullPath,
				isDir:      false,
			}
			f.curSize += size
			f.mu.Unlock()
		}
	}

	// After scan, check if we need to free up space
	return f.ensureSpace(ctx, dirPath, 0)
}

// updateFileSize updates the size of a file in our tracking
func (f *FileSystem) updateFileSize(path string, size int64, isDir bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	existingInfo, exists := f.files[path]
	if exists {
		// Update size only if it's not a directory
		if !existingInfo.isDir {
			// Update current size
			f.curSize = f.curSize - existingInfo.size + size
			existingInfo.size = size
		}
	} else {
		// Add new file to tracking
		var fileSize int64
		if isDir {
			fileSize = 0
		} else {
			fileSize = size
		}

		f.files[path] = &fileInfo{
			size:       fileSize,
			lastAccess: time.Now(),
			path:       path,
			isDir:      isDir,
		}
		if !isDir {
			f.curSize += size
		}
	}
}

// updateAccessTime updates the last access time for a file
func (f *FileSystem) updateAccessTime(path string) {
	f.mu.RLock()
	info, exists := f.files[path]
	f.mu.RUnlock()

	if exists {
		f.mu.Lock()
		info.lastAccess = time.Now()
		f.mu.Unlock()
	}
}

// ensureSpace ensures there's enough space for a file of the given size
// by removing least recently accessed files if necessary
func (f *FileSystem) ensureSpace(ctx context.Context, name string, additionalSize int64) error {
	// Quick check with read lock first
	f.mu.RLock()
	needCleanup := f.curSize+additionalSize > f.maxSize
	f.mu.RUnlock()

	if !needCleanup {
		return nil
	}

	// If we need cleanup, acquire write lock
	f.mu.Lock()

	// Re-check after acquiring write lock
	if f.curSize+additionalSize <= f.maxSize {
		f.mu.Unlock()
		return nil
	}

	// Get a list of files sorted by access time (oldest first)
	var files []*fileInfo
	for _, info := range f.files {
		if !info.isDir && info.size > 0 { // Only include non-empty files
			files = append(files, info)
		}
	}

	// Release lock while sorting
	f.mu.Unlock()

	// Sort files by access time
	sort.Slice(files, func(i, j int) bool {
		return files[i].lastAccess.Before(files[j].lastAccess)
	})

	// Free up space until we have enough or run out of files to delete
	var lastError error

	for _, info := range files {
		// Skip directories and empty files
		if info.isDir || info.size == 0 {
			continue
		}

		// Check if we still need to remove this file
		f.mu.RLock()
		stillNeedRemoval := f.curSize+additionalSize > f.maxSize
		f.mu.RUnlock()

		if !stillNeedRemoval {
			break
		}

		// Try to remove the file
		removeErr := f.fs.RemoveAll(ctx, info.path)
		if removeErr != nil {
			lastError = removeErr
			continue
		}

		// Update tracking after successful removal
		f.mu.Lock()
		// Double-check the file is still in our tracking (might have been removed by another operation)
		if fileInfo, exists := f.files[info.path]; exists {
			if !fileInfo.isDir {
				f.curSize -= fileInfo.size
			}
			delete(f.files, info.path)
		}
		f.mu.Unlock()
	}

	// Check if we've freed up enough space
	f.mu.RLock()
	success := f.curSize+additionalSize <= f.maxSize
	f.mu.RUnlock()

	if !success && lastError != nil {
		return errors.Wrap(lastError, "failed to free up enough space")
	}

	if !success {
		return &os.PathError{
			Op:   "write",
			Path: name,
			Err:  syscall.ENOSPC,
		}
	}

	return nil
}

// Close implements webdav.File.
func (f *File) Close() error {
	// Get file info before closing to capture final size
	info, err := f.file.Stat()
	if err == nil {
		f.fs.updateFileSize(f.path, info.Size(), info.IsDir())
	}
	return f.file.Close()
}

// Read implements webdav.File.
func (f *File) Read(p []byte) (n int, err error) {
	n, err = f.file.Read(p)
	if n > 0 {
		// Update access time on successful read
		f.fs.updateAccessTime(f.path)
	}
	return n, err
}

// Seek implements webdav.File.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

// Readdir implements webdav.File.
func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	return f.file.Readdir(count)
}

// Stat implements webdav.File.
func (f *File) Stat() (os.FileInfo, error) {
	info, err := f.file.Stat()
	if err == nil {
		// Update access time on successful stat
		f.fs.updateAccessTime(f.path)
	}
	return info, err
}

// Write implements webdav.File.
func (f *File) Write(p []byte) (n int, err error) {
	// Get current file information to estimate new size
	curInfo, statErr := f.file.Stat()

	if statErr == nil && !curInfo.IsDir() {
		// If we can get current size, estimate new size
		curPos, seekErr := f.file.Seek(0, io.SeekCurrent)
		if seekErr == nil {
			// Calculate how much the file will grow
			currentSize := curInfo.Size()
			expectedSize := currentSize
			if curPos+int64(len(p)) > currentSize {
				expectedSize = curPos + int64(len(p))
			}

			// Calculate additional size needed
			additionalSize := expectedSize - currentSize
			if additionalSize > 0 {
				// Ensure we have enough space for the estimated new size
				if err := f.fs.ensureSpace(f.ctx, f.path, additionalSize); err != nil {
					return 0, errors.Wrap(err, "failed to ensure space for write operation")
				}
			}
		}
	}

	// Perform the write
	n, err = f.file.Write(p)
	if n > 0 {
		// Update access time on successful write
		f.fs.updateAccessTime(f.path)
	}
	return n, err
}

// NewFileSystem creates a new size-capped filesystem
func NewFileSystem(fs webdav.FileSystem, maxSize int64) *FileSystem {
	return &FileSystem{
		fs:      fs,
		maxSize: maxSize,
		files:   make(map[string]*fileInfo),
		curSize: 0,
	}
}

var _ webdav.FileSystem = &FileSystem{}
var _ webdav.File = &File{}
