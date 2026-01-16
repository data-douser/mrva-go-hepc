// Package storage defines the interface for CodeQL database storage backends.
package storage

import (
	"context"
	"io"

	"github.com/data-douser/mrva-go-hepc/api"
)

// Backend represents a storage backend for CodeQL databases.
// Implementations must provide methods to list available databases
// and retrieve database files.
type Backend interface {
	// Type returns the storage backend type identifier (e.g., "local", "gcs").
	Type() string

	// ListMetadata returns all database metadata records from the storage backend.
	// The metadata is typically stored in a SQLite database (metadata.sql) or
	// equivalent storage mechanism.
	ListMetadata(ctx context.Context) ([]api.DatabaseMetadata, error)

	// GetFile retrieves a database file by name and returns a ReadCloser.
	// The caller is responsible for closing the returned reader.
	// Returns the file size and content type along with the reader.
	GetFile(ctx context.Context, filename string) (io.ReadCloser, int64, string, error)

	// FileExists checks if a file exists in the storage backend.
	FileExists(ctx context.Context, filename string) (bool, error)

	// MetadataExists checks if the metadata database exists and is accessible.
	MetadataExists(ctx context.Context) (bool, error)

	// Close releases any resources held by the storage backend.
	Close() error
}

// Config holds common configuration for storage backends.
type Config struct {
	// MetadataFile is the name of the metadata database file (default: "metadata.sql").
	MetadataFile string
}

// DefaultConfig returns the default storage configuration.
func DefaultConfig() Config {
	return Config{
		MetadataFile: "metadata.sql",
	}
}

// ErrNotFound is returned when a requested file does not exist.
type ErrNotFound struct {
	Path string
}

func (e *ErrNotFound) Error() string {
	return "file not found: " + e.Path
}

// ErrMetadataNotFound is returned when the metadata database is not found.
type ErrMetadataNotFound struct {
	Path string
}

func (e *ErrMetadataNotFound) Error() string {
	return "metadata database not found: " + e.Path
}
