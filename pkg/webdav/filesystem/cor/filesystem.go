package cor

import (
	"context"
	"io"
	"os"
	"path"
	"sync"

	"golang.org/x/net/webdav"
)

// FileSystem implements webdav.FileSystem with Copy-on-Read behavior
type FileSystem struct {
	// Cache filesystem used for primary reads
	cache webdav.FileSystem

	// Backend filesystem used as the source of truth
	backend webdav.FileSystem

	// Cache for directory listings using sync.Map for concurrent access
	dirCache sync.Map // map[string][]os.FileInfo
}

// Mkdir implements webdav.FileSystem.
func (f *FileSystem) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	// Create directory on both cache and backend
	err := f.backend.Mkdir(ctx, name, perm)
	if err != nil {
		return err
	}

	// Create in cache as well (ignore error if it exists)
	_ = f.cache.Mkdir(ctx, name, perm)

	// Invalidate parent directory cache
	f.invalidateDirectoryCache(path.Dir(name))

	return nil
}

// OpenFile implements webdav.FileSystem.
func (f *FileSystem) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	// Check if this is a write operation
	isWriting := flag&(os.O_WRONLY|os.O_RDWR|os.O_APPEND|os.O_CREATE|os.O_TRUNC) != 0

	if isWriting {
		// For write operations, open on both backend and cache
		backendFile, err := f.backend.OpenFile(ctx, name, flag, perm)
		if err != nil {
			return nil, err
		}

		// Attempt to open or create on cache as well
		cacheFile, err := f.cache.OpenFile(ctx, name, flag, perm)
		if err != nil {
			// If we can't open the cache file, just close the backend and return error
			backendFile.Close()
			return nil, err
		}

		// We successfully opened both backend and cache files for writing
		// Return a special file that writes to both
		return &writeThroughFile{
			fs:          f,
			name:        name,
			backendFile: backendFile,
			cacheFile:   cacheFile,
		}, nil
	}

	// For read operations, try cache first
	cacheFile, err := f.cache.OpenFile(ctx, name, flag, perm)
	if err == nil {
		// File exists in cache, use it
		return &File{
			file:      cacheFile,
			fs:        f,
			name:      name,
			fromCache: true,
		}, nil
	}

	// File not in cache, try backend
	backendFile, err := f.backend.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	// Get file info to check if it's a directory
	info, err := backendFile.Stat()
	if err != nil {
		backendFile.Close()
		return nil, err
	}

	if info.IsDir() {
		// For directories, no special handling needed
		return &File{
			file:      backendFile,
			fs:        f,
			name:      name,
			fromCache: false,
		}, nil
	}

	// For regular files, copy to cache and then serve
	if err := f.copyToCache(ctx, name, backendFile, info); err != nil {
		// If copying to cache fails, just use the backend file directly
		return &File{
			file:      backendFile,
			fs:        f,
			name:      name,
			fromCache: false,
		}, nil
	}

	// Close the backend file since we've copied its contents
	backendFile.Close()

	// Reopen from cache
	cacheFile, err = f.cache.OpenFile(ctx, name, flag, perm)
	if err != nil {
		// If reopening from cache fails, reopen from backend
		backendFile, err = f.backend.OpenFile(ctx, name, flag, perm)
		if err != nil {
			return nil, err
		}
		return &File{
			file:      backendFile,
			fs:        f,
			name:      name,
			fromCache: false,
		}, nil
	}

	return &File{
		file:      cacheFile,
		fs:        f,
		name:      name,
		fromCache: true,
	}, nil
}

// RemoveAll implements webdav.FileSystem.
func (f *FileSystem) RemoveAll(ctx context.Context, name string) error {
	// Remove from backend first
	err := f.backend.RemoveAll(ctx, name)
	if err != nil {
		return err
	}

	// Remove from cache as well (ignore errors)
	_ = f.cache.RemoveAll(ctx, name)

	// Invalidate parent directory cache
	f.invalidateDirectoryCache(path.Dir(name))

	return nil
}

// Rename implements webdav.FileSystem.
func (f *FileSystem) Rename(ctx context.Context, oldName string, newName string) error {
	// Rename on backend first
	err := f.backend.Rename(ctx, oldName, newName)
	if err != nil {
		return err
	}

	// Rename on cache as well (ignore errors)
	_ = f.cache.Rename(ctx, oldName, newName)

	// Invalidate parent directory caches for both old and new paths
	f.invalidateDirectoryCache(path.Dir(oldName))
	if path.Dir(oldName) != path.Dir(newName) {
		f.invalidateDirectoryCache(path.Dir(newName))
	}

	return nil
}

// Stat implements webdav.FileSystem.
func (f *FileSystem) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	// Try stat from cache first
	info, err := f.cache.Stat(ctx, name)
	if err == nil {
		return info, nil
	}

	// If not in cache, get from backend
	info, err = f.backend.Stat(ctx, name)
	if err != nil {
		return nil, err
	}

	// For directories, no additional handling needed
	if info.IsDir() {
		return info, nil
	}

	// For files, we should make sure they exist in cache for future use
	// but do it asynchronously to not block the Stat call
	go func() {
		backendFile, err := f.backend.OpenFile(ctx, name, os.O_RDONLY, 0)
		if err != nil {
			return
		}
		defer backendFile.Close()

		_ = f.copyToCache(ctx, name, backendFile, info)
	}()

	return info, nil
}

// NewFileSystem creates a new Copy-on-Read filesystem
func NewFileSystem(cache webdav.FileSystem, backend webdav.FileSystem) *FileSystem {
	return &FileSystem{
		cache:   cache,
		backend: backend,
		// sync.Map doesn't need initialization
	}
}

// Helper methods for file operations

// copyToCache copies a file from backend to cache
func (f *FileSystem) copyToCache(ctx context.Context, name string, backendFile webdav.File, info os.FileInfo) error {
	// Create all parent directories in cache
	dir := path.Dir(name)
	if dir != "." && dir != "/" {
		err := f.ensureDirectory(ctx, dir)
		if err != nil {
			return err
		}
	}

	// Create file in cache
	cacheFile, err := f.cache.OpenFile(ctx, name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
	if err != nil {
		return err
	}
	defer cacheFile.Close()

	// Reset backend file position
	_, err = backendFile.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	// Copy content from backend to cache
	_, err = io.Copy(cacheFile, backendFile)
	if err != nil {
		return err
	}

	return nil
}

// ensureDirectory creates a directory and all its parents in the cache
func (f *FileSystem) ensureDirectory(ctx context.Context, dir string) error {
	// Try to stat the directory first
	_, err := f.cache.Stat(ctx, dir)
	if err == nil {
		// Directory exists
		return nil
	}

	// Create parent directories first
	parent := path.Dir(dir)
	if parent != "." && parent != "/" && parent != dir {
		err := f.ensureDirectory(ctx, parent)
		if err != nil {
			return err
		}
	}

	// Create the directory in cache
	err = f.cache.Mkdir(ctx, dir, 0755)
	if err != nil && !os.IsExist(err) {
		return err
	}

	return nil
}

// getCachedDirectoryListing retrieves a cached directory listing
func (f *FileSystem) getCachedDirectoryListing(name string) ([]os.FileInfo, bool) {
	value, ok := f.dirCache.Load(name)
	if !ok {
		return nil, false
	}
	entries, ok := value.([]os.FileInfo)
	return entries, ok
}

// cacheDirectoryListing caches a directory listing
func (f *FileSystem) cacheDirectoryListing(name string, entries []os.FileInfo) {
	f.dirCache.Store(name, entries)
}

// invalidateDirectoryCache removes a directory listing from the cache
func (f *FileSystem) invalidateDirectoryCache(name string) {
	f.dirCache.Delete(name)
}

// writeThroughFile is a special file that writes to both backend and cache
type writeThroughFile struct {
	fs          *FileSystem
	name        string
	backendFile webdav.File
	cacheFile   webdav.File
}

func (f *writeThroughFile) Close() error {
	// Close both files, prefer to return backend error if any
	cacheErr := f.cacheFile.Close()
	backendErr := f.backendFile.Close()

	// Invalidate parent directory cache
	f.fs.invalidateDirectoryCache(path.Dir(f.name))

	if backendErr != nil {
		return backendErr
	}
	return cacheErr
}

func (f *writeThroughFile) Read(p []byte) (n int, err error) {
	// Prefer to read from backend for consistency
	return f.backendFile.Read(p)
}

func (f *writeThroughFile) Seek(offset int64, whence int) (int64, error) {
	// Seek in both files, prefer to return backend position and error
	_, cacheErr := f.cacheFile.Seek(offset, whence)
	backendPos, backendErr := f.backendFile.Seek(offset, whence)

	if backendErr != nil {
		return backendPos, backendErr
	}
	if cacheErr != nil {
		// If cache seek fails but backend succeeds, try to keep them in sync
		_, _ = f.cacheFile.Seek(backendPos, io.SeekStart)
	}
	return backendPos, nil
}

func (f *writeThroughFile) Readdir(count int) ([]os.FileInfo, error) {
	// Use backend for directory listing
	entries, err := f.backendFile.Readdir(count)
	if err != nil {
		return nil, err
	}

	// Cache the result
	f.fs.cacheDirectoryListing(f.name, entries)

	return entries, nil
}

func (f *writeThroughFile) Stat() (os.FileInfo, error) {
	// Prefer backend stat for consistency
	return f.backendFile.Stat()
}

func (f *writeThroughFile) Write(p []byte) (n int, err error) {
	// Write to backend first
	n, err = f.backendFile.Write(p)
	if err != nil {
		return n, err
	}

	// Then write the same data to cache
	_, cacheErr := f.cacheFile.Write(p)
	if cacheErr != nil {
		// If cache write fails, log but don't fail the operation
		// TODO: Add proper logging
	}

	return n, nil
}

var _ webdav.File = &writeThroughFile{}
var _ webdav.FileSystem = &FileSystem{}
