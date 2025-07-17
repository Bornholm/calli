package cor

import (
	"io/fs"

	"golang.org/x/net/webdav"
)

// File implements webdav.File for the Copy-on-Read filesystem
type File struct {
	// The underlying file from either cache or backend
	file webdav.File

	// Reference to parent filesystem for caching operations
	fs *FileSystem

	// File path
	name string

	// Flag to indicate whether this file is from cache or backend
	fromCache bool
}

// Close implements webdav.File.
func (f *File) Close() error {
	return f.file.Close()
}

// Read implements webdav.File.
func (f *File) Read(p []byte) (n int, err error) {
	return f.file.Read(p)
}

// Readdir implements webdav.File.
func (f *File) Readdir(count int) ([]fs.FileInfo, error) {
	// Check if there's a cached directory listing
	entries, ok := f.fs.getCachedDirectoryListing(f.name)
	if ok {
		return entries, nil
	}

	// Get directory listing from the file
	entries, err := f.file.Readdir(count)
	if err != nil {
		return nil, err
	}

	// Cache the directory listing
	f.fs.cacheDirectoryListing(f.name, entries)

	return entries, nil
}

// Seek implements webdav.File.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	return f.file.Seek(offset, whence)
}

// Stat implements webdav.File.
func (f *File) Stat() (fs.FileInfo, error) {
	return f.file.Stat()
}

// Write implements webdav.File.
func (f *File) Write(p []byte) (n int, err error) {
	// Write to the underlying file
	n, err = f.file.Write(p)
	if err != nil {
		return n, err
	}
	
	// Invalidate directory cache for parent directory
	// Note: This is redundant with our writeThroughFile implementation,
	// but we include it here for completeness in case direct File writes happen
	if !f.fromCache {
		f.fs.invalidateDirectoryCache(f.name)
	}
	
	return n, nil
}

var _ webdav.File = &File{}