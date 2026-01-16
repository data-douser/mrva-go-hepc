// Package local provides a local filesystem storage backend for CodeQL databases.
package local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/data-douser/mrva-go-hepc/api"
	"github.com/data-douser/mrva-go-hepc/internal/codeql"
	"github.com/data-douser/mrva-go-hepc/internal/storage"
)

// Backend implements storage.Backend for local filesystem storage.
type Backend struct {
	basePath    string
	endpointURL string

	// Cache for discovered databases
	mu             sync.RWMutex
	cachedMetadata []api.DatabaseMetadata
	cacheTime      time.Time
	cacheTTL       time.Duration
	discoveredDBs  map[string]*codeql.DiscoveredDatabase // keyed by content hash or path
}

// Config holds configuration for the local storage backend.
type Config struct {
	// BasePath is the directory containing CodeQL databases.
	BasePath string

	// EndpointURL is the base URL for constructing result URLs (e.g., "http://localhost:8080").
	EndpointURL string

	// CacheTTL is how long to cache discovered metadata (default: 5 minutes).
	CacheTTL time.Duration
}

// New creates a new local filesystem storage backend.
func New(cfg Config) (*Backend, error) {
	if cfg.BasePath == "" {
		return nil, fmt.Errorf("local storage: base path is required")
	}

	// Verify the base path exists and is a directory
	info, err := os.Stat(cfg.BasePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("local storage: directory does not exist: %s", cfg.BasePath)
		}
		return nil, fmt.Errorf("local storage: cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("local storage: not a directory: %s", cfg.BasePath)
	}

	endpointURL := cfg.EndpointURL
	if endpointURL == "" {
		endpointURL = "http://localhost:8080"
	}

	cacheTTL := cfg.CacheTTL
	if cacheTTL == 0 {
		cacheTTL = 5 * time.Minute
	}

	return &Backend{
		basePath:      cfg.BasePath,
		endpointURL:   endpointURL,
		cacheTTL:      cacheTTL,
		discoveredDBs: make(map[string]*codeql.DiscoveredDatabase),
	}, nil
}

// Type returns the storage backend type identifier.
func (b *Backend) Type() string {
	return "local"
}

// ListMetadata returns all database metadata records by discovering CodeQL databases.
func (b *Backend) ListMetadata(ctx context.Context) ([]api.DatabaseMetadata, error) {
	// Check if we have valid cached data
	b.mu.RLock()
	if b.cachedMetadata != nil && time.Since(b.cacheTime) < b.cacheTTL {
		result := make([]api.DatabaseMetadata, len(b.cachedMetadata))
		copy(result, b.cachedMetadata)
		b.mu.RUnlock()
		return result, nil
	}
	b.mu.RUnlock()

	// Discover databases
	databases, err := codeql.DiscoverDatabases(b.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to discover databases: %w", err)
	}

	// Convert to API metadata format
	var metadata []api.DatabaseMetadata
	discoveredMap := make(map[string]*codeql.DiscoveredDatabase)

	for _, db := range databases {
		m := b.convertToMetadata(db)
		metadata = append(metadata, m)

		// Index by content hash for archived, by path for unarchived
		if db.ContentHash != "" {
			discoveredMap[db.ContentHash] = db
		} else {
			discoveredMap[db.Path] = db
		}
	}

	// Update cache
	b.mu.Lock()
	b.cachedMetadata = metadata
	b.cacheTime = time.Now()
	b.discoveredDBs = discoveredMap
	b.mu.Unlock()

	return metadata, nil
}

// convertToMetadata converts a discovered database to API metadata format.
func (b *Backend) convertToMetadata(db *codeql.DiscoveredDatabase) api.DatabaseMetadata {
	// Generate content hash for unarchived databases
	contentHash := db.ContentHash
	if contentHash == "" {
		// For unarchived databases, create a hash from the path
		h := sha256.Sum256([]byte(db.Path))
		contentHash = hex.EncodeToString(h[:])
	}

	// Build CID from available metadata
	buildCID := ""
	creationTime := ""
	cliVersion := ""
	sourceSHA := ""

	if db.CreationMetadata != nil {
		cliVersion = db.CreationMetadata.CLIVersion
		creationTime = db.CreationMetadata.CreationTime
		sourceSHA = db.CreationMetadata.SHA
		buildCID = generateBuildCID(cliVersion, creationTime, db.Language, sourceSHA)
	} else {
		// Generate a build CID from available info
		buildCID = contentHash[:10]
	}

	// Construct the result URL
	relPath, err := filepath.Rel(b.basePath, db.Path)
	if err != nil {
		// Fallback to base name if relative path fails
		relPath = filepath.Base(db.Path)
	}
	resultURL := fmt.Sprintf("%s/db/%s", strings.TrimSuffix(b.endpointURL, "/"), relPath)

	// Determine tool name
	toolName := "codeql"
	if db.Language != "" && db.Language != "unknown" {
		toolName = fmt.Sprintf("codeql-%s", db.Language)
	}

	return api.DatabaseMetadata{
		ContentHash:          contentHash,
		BuildCID:             buildCID,
		GitBranch:            "HEAD",
		GitCommitID:          sourceSHA,
		GitOwner:             db.Owner,
		GitRepo:              db.Repo,
		IngestionDatetimeUTC: creationTime,
		PrimaryLanguage:      db.Language,
		ResultURL:            resultURL,
		ToolName:             toolName,
		ToolVersion:          cliVersion,
		Projname:             fmt.Sprintf("%s/%s", db.Owner, db.Repo),
		DBFileSize:           db.FileSize,
	}
}

// generateBuildCID creates a build context identifier.
func generateBuildCID(cliVersion, creationTime, language, sourceSHA string) string {
	s := fmt.Sprintf("%s %s %s %s", cliVersion, creationTime, language, sourceSHA)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:10]
}

// GetFile retrieves a database file by path from the local filesystem.
func (b *Backend) GetFile(ctx context.Context, filename string) (io.ReadCloser, int64, string, error) {
	// Resolve the path - it could be a relative path from the result URL
	var fullPath string
	if filepath.IsAbs(filename) {
		fullPath = filename
	} else {
		fullPath = filepath.Join(b.basePath, filename)
	}

	// Security: ensure the path is within the base path
	fullPath = filepath.Clean(fullPath)
	if !strings.HasPrefix(fullPath, filepath.Clean(b.basePath)) {
		return nil, 0, "", fmt.Errorf("access denied: path outside base directory")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, 0, "", &storage.ErrNotFound{Path: fullPath}
		}
		return nil, 0, "", fmt.Errorf("error accessing file: %w", err)
	}

	if info.IsDir() {
		return nil, 0, "", fmt.Errorf("cannot serve directory: %s", fullPath)
	}

	file, err := os.Open(fullPath)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to open file: %w", err)
	}

	// Determine content type from extension
	contentType := mime.TypeByExtension(filepath.Ext(fullPath))
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return file, info.Size(), contentType, nil
}

// FileExists checks if a file exists in the local filesystem.
func (b *Backend) FileExists(ctx context.Context, filename string) (bool, error) {
	var fullPath string
	if filepath.IsAbs(filename) {
		fullPath = filename
	} else {
		fullPath = filepath.Join(b.basePath, filename)
	}

	fullPath = filepath.Clean(fullPath)
	if !strings.HasPrefix(fullPath, filepath.Clean(b.basePath)) {
		return false, fmt.Errorf("access denied: path outside base directory")
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// MetadataExists always returns true for local storage since we discover dynamically.
func (b *Backend) MetadataExists(ctx context.Context) (bool, error) {
	// We can always discover databases dynamically
	return true, nil
}

// Close releases any resources held by the backend.
func (b *Backend) Close() error {
	// Clear the cache
	b.mu.Lock()
	b.cachedMetadata = nil
	b.discoveredDBs = nil
	b.mu.Unlock()
	return nil
}

// BasePath returns the base path of the local storage backend.
func (b *Backend) BasePath() string {
	return b.basePath
}

// InvalidateCache forces a refresh of the discovered databases cache.
func (b *Backend) InvalidateCache() {
	b.mu.Lock()
	b.cachedMetadata = nil
	b.cacheTime = time.Time{}
	b.mu.Unlock()
}
