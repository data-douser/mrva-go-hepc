// Package gcs provides a Google Cloud Storage backend for CodeQL databases.
package gcs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/storage"
	"github.com/data-douser/mrva-go-hepc/api"
	hepcStorage "github.com/data-douser/mrva-go-hepc/internal/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	"gopkg.in/yaml.v3"
)

// Backend implements storage.Backend for Google Cloud Storage.
type Backend struct {
	client        *storage.Client
	bucket        string
	prefix        string
	localCacheDir string
	endpointURL   string

	// Cache for discovered databases
	mu             sync.RWMutex
	cachedMetadata []api.DatabaseMetadata
	cacheTime      time.Time
	cacheTTL       time.Duration
}

// Config holds configuration for the GCS storage backend.
type Config struct {
	// Bucket is the GCS bucket name (required).
	Bucket string

	// Prefix is an optional path prefix within the bucket.
	Prefix string

	// CredentialsFile is the path to a service account JSON key file.
	// If empty, uses Application Default Credentials (ADC).
	CredentialsFile string

	// LocalCacheDir is a local directory for caching downloaded files.
	// If empty, uses a temp directory.
	LocalCacheDir string

	// EndpointURL is the base URL for constructing result URLs.
	EndpointURL string

	// CacheTTL is how long to cache discovered metadata (default: 5 minutes).
	CacheTTL time.Duration

	// Client is an optional pre-configured GCS client for testing.
	// If provided, CredentialsFile is ignored.
	Client *storage.Client
}

// New creates a new GCS storage backend.
func New(ctx context.Context, cfg Config) (*Backend, error) {
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("gcs storage: bucket name is required")
	}

	var client *storage.Client
	var err error

	// Use provided client (for testing) or create a new one
	if cfg.Client != nil {
		client = cfg.Client
	} else {
		var opts []option.ClientOption
		if cfg.CredentialsFile != "" {
			credsData, readErr := os.ReadFile(cfg.CredentialsFile)
			if readErr != nil {
				return nil, fmt.Errorf("gcs storage: failed to read credentials file: %w", readErr)
			}
			//nolint:staticcheck // SA1019: option.WithCredentialsJSON is deprecated, but still needed for service account support
			opts = append(opts, option.WithCredentialsJSON(credsData))
		}

		client, err = storage.NewClient(ctx, opts...)
		if err != nil {
			return nil, fmt.Errorf("gcs storage: failed to create client: %w", err)
		}
	}

	prefix := cfg.Prefix
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	localCacheDir := cfg.LocalCacheDir
	if localCacheDir == "" {
		localCacheDir, err = os.MkdirTemp("", "hepc-gcs-cache-*")
		if err != nil {
			_ = client.Close() //nolint:errcheck // Best effort close
			return nil, fmt.Errorf("gcs storage: failed to create cache directory: %w", err)
		}
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
		client:        client,
		bucket:        cfg.Bucket,
		prefix:        prefix,
		localCacheDir: localCacheDir,
		endpointURL:   endpointURL,
		cacheTTL:      cacheTTL,
	}, nil
}

// Type returns the storage backend type identifier.
func (b *Backend) Type() string {
	return "gcs"
}

// objectPath returns the full object path including prefix.
func (b *Backend) objectPath(filename string) string {
	return b.prefix + filename
}

// ListMetadata discovers CodeQL databases in the GCS bucket and returns their metadata.
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
	metadata, err := b.discoverDatabases(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to discover databases: %w", err)
	}

	// Update cache
	b.mu.Lock()
	b.cachedMetadata = metadata
	b.cacheTime = time.Now()
	b.mu.Unlock()

	return metadata, nil
}

// discoverDatabases scans the GCS bucket for unarchived CodeQL databases.
// Note: Archived (.zip) databases are not supported in cloud storage because
// reading metadata would require downloading the entire archive, which is
// unacceptable for large databases (often multiple gigabytes).
func (b *Backend) discoverDatabases(ctx context.Context) ([]api.DatabaseMetadata, error) {
	var metadata []api.DatabaseMetadata

	// Track discovered databases to avoid duplicates
	discovered := make(map[string]bool)

	bkt := b.client.Bucket(b.bucket)
	query := &storage.Query{Prefix: b.prefix}

	it := bkt.Objects(ctx, query)
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		objectName := attrs.Name

		// Only support unarchived databases (look for codeql-database.yml)
		// Archived (.zip) databases are skipped - reading metadata would require
		// downloading the entire archive which is not acceptable for large databases.
		if strings.HasSuffix(objectName, "/codeql-database.yml") {
			// Extract the database directory path
			dbPath := strings.TrimSuffix(objectName, "/codeql-database.yml")
			if discovered[dbPath] {
				continue
			}
			discovered[dbPath] = true

			m, err := b.discoverUnarchivedDatabase(ctx, dbPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", dbPath, err)
				continue
			}
			if m != nil {
				metadata = append(metadata, *m)
			}
		}
	}

	return metadata, nil
}

// discoverUnarchivedDatabase extracts metadata from an unarchived CodeQL database in GCS.
func (b *Backend) discoverUnarchivedDatabase(ctx context.Context, dbPath string) (*api.DatabaseMetadata, error) {
	// Download codeql-database.yml
	ymlContent, err := b.readObject(ctx, dbPath+"/codeql-database.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to read codeql-database.yml: %w", err)
	}

	var dbYAML databaseYAML
	if err := yaml.Unmarshal(ymlContent, &dbYAML); err != nil {
		return nil, fmt.Errorf("failed to parse codeql-database.yml: %w", err)
	}

	// Determine language
	language := dbYAML.PrimaryLanguage
	if language == "" {
		language = b.detectLanguageFromGCS(ctx, dbPath)
	}

	// Skip directory size calculation for performance - would require listing
	// all objects in the database directory which is slow for large databases.
	// Use 0 as a placeholder; consumers should not rely on this value for GCS storage.
	var totalSize int64 = 0

	// Generate content hash from path
	h := sha256.Sum256([]byte(dbPath))
	contentHash := hex.EncodeToString(h[:])

	// Extract owner/repo
	owner, repo := extractOwnerRepo(dbYAML.SourceLocationPrefix)

	// Build result URL
	relPath := strings.TrimPrefix(dbPath, b.prefix)
	resultURL := fmt.Sprintf("%s/db/%s", strings.TrimSuffix(b.endpointURL, "/"), relPath)

	// Build metadata
	buildCID := contentHash[:10]
	creationTime := ""
	cliVersion := ""
	sourceSHA := ""

	if dbYAML.CreationMetadata != nil {
		cliVersion = dbYAML.CreationMetadata.CLIVersion
		creationTime = dbYAML.CreationMetadata.CreationTime
		sourceSHA = dbYAML.CreationMetadata.SHA
		buildCID = generateBuildCID(cliVersion, creationTime, language, sourceSHA)
	}

	toolName := "codeql"
	if language != "" && language != "unknown" {
		toolName = fmt.Sprintf("codeql-%s", language)
	}

	return &api.DatabaseMetadata{
		ContentHash:          contentHash,
		BuildCID:             buildCID,
		GitBranch:            "HEAD",
		GitCommitID:          sourceSHA,
		GitOwner:             owner,
		GitRepo:              repo,
		IngestionDatetimeUTC: creationTime,
		PrimaryLanguage:      language,
		ResultURL:            resultURL,
		ToolName:             toolName,
		ToolVersion:          cliVersion,
		Projname:             fmt.Sprintf("%s/%s", owner, repo),
		DBFileSize:           totalSize,
	}, nil
}

// readObject reads the contents of an object from GCS.
func (b *Backend) readObject(ctx context.Context, objectName string) ([]byte, error) {
	obj := b.client.Bucket(b.bucket).Object(objectName)
	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = reader.Close() //nolint:errcheck // Best effort close
	}()

	return io.ReadAll(reader)
}

// downloadObject downloads an object from GCS to a local file.
// detectLanguageFromGCS detects the language by looking for db-<lang> directories.
func (b *Backend) detectLanguageFromGCS(ctx context.Context, dbPath string) string {
	bkt := b.client.Bucket(b.bucket)
	query := &storage.Query{Prefix: dbPath + "/db-"}

	it := bkt.Objects(ctx, query)
	attrs, err := it.Next()
	if err != nil {
		return "unknown"
	}

	// Extract language from path like "ex1/minio-db/db-go/..."
	parts := strings.Split(attrs.Name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "db-") {
			return strings.TrimPrefix(part, "db-")
		}
	}

	return "unknown"
}

// GetFile retrieves a database file from GCS.
func (b *Backend) GetFile(ctx context.Context, filename string) (io.ReadCloser, int64, string, error) {
	objectName := b.objectPath(filename)
	obj := b.client.Bucket(b.bucket).Object(objectName)

	attrs, err := obj.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, 0, "", &hepcStorage.ErrNotFound{Path: objectName}
		}
		return nil, 0, "", fmt.Errorf("failed to get object attributes: %w", err)
	}

	reader, err := obj.NewReader(ctx)
	if err != nil {
		return nil, 0, "", fmt.Errorf("failed to create reader: %w", err)
	}

	contentType := attrs.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	return reader, attrs.Size, contentType, nil
}

// FileExists checks if a file exists in GCS.
func (b *Backend) FileExists(ctx context.Context, filename string) (bool, error) {
	objectName := b.objectPath(filename)
	obj := b.client.Bucket(b.bucket).Object(objectName)

	_, err := obj.Attrs(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// MetadataExists always returns true for GCS storage since we discover dynamically.
func (b *Backend) MetadataExists(ctx context.Context) (bool, error) {
	return true, nil
}

// Close releases any resources held by the backend.
func (b *Backend) Close() error {
	b.mu.Lock()
	b.cachedMetadata = nil
	b.mu.Unlock()

	return b.client.Close()
}

// Helper types for YAML parsing

type databaseYAML struct {
	SourceLocationPrefix string                    `yaml:"sourceLocationPrefix"`
	PrimaryLanguage      string                    `yaml:"primaryLanguage"`
	CreationMetadata     *databaseCreationMetadata `yaml:"creationMetadata,omitempty"`
}

type databaseCreationMetadata struct {
	SHA          string `yaml:"sha"`
	CLIVersion   string `yaml:"cliVersion"`
	CreationTime string `yaml:"creationTime"`
}

// Helper functions

func extractOwnerRepo(sourceLocationPrefix string) (owner, repo string) {
	if sourceLocationPrefix == "" {
		return "unknown", "unknown"
	}

	parts := strings.Split(filepath.Clean(sourceLocationPrefix), string(filepath.Separator))
	var filteredParts []string
	for _, p := range parts {
		if p != "" {
			filteredParts = append(filteredParts, p)
		}
	}

	switch {
	case len(filteredParts) >= 2:
		repo = filteredParts[len(filteredParts)-1]
		owner = filteredParts[len(filteredParts)-2]
	case len(filteredParts) == 1:
		repo = filteredParts[0]
		owner = "unknown"
	default:
		owner = "unknown"
		repo = "unknown"
	}

	return owner, repo
}

func generateBuildCID(cliVersion, creationTime, language, sourceSHA string) string {
	s := fmt.Sprintf("%s %s %s %s", cliVersion, creationTime, language, sourceSHA)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])[:10]
}
