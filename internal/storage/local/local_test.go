// Package local provides a local filesystem storage backend for CodeQL databases.
package local

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/data-douser/mrva-go-hepc/api"
	"github.com/data-douser/mrva-go-hepc/internal/codeql"
	"github.com/data-douser/mrva-go-hepc/internal/storage"
)

func TestNew(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-storage-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name      string
		cfg       Config
		wantErr   bool
		errSubstr string
	}{
		{
			name: "valid config",
			cfg: Config{
				BasePath:    tempDir,
				EndpointURL: "http://localhost:8080",
			},
			wantErr: false,
		},
		{
			name: "empty base path",
			cfg: Config{
				BasePath: "",
			},
			wantErr:   true,
			errSubstr: "base path is required",
		},
		{
			name: "non-existent directory",
			cfg: Config{
				BasePath: "/nonexistent/path/to/directory",
			},
			wantErr:   true,
			errSubstr: "does not exist",
		},
		{
			name: "path is a file not directory",
			cfg: Config{
				BasePath: func() string {
					f, _ := os.CreateTemp(tempDir, "file-*")
					name := f.Name()
					f.Close()
					return name
				}(),
			},
			wantErr:   true,
			errSubstr: "not a directory",
		},
		{
			name: "default endpoint URL",
			cfg: Config{
				BasePath: tempDir,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend, err := New(tt.cfg)

			if tt.wantErr {
				if err == nil {
					t.Error("New() expected error, got nil")
				} else if tt.errSubstr != "" && !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("New() error = %q, want error containing %q", err.Error(), tt.errSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("New() unexpected error: %v", err)
			}
			defer backend.Close()

			if backend.Type() != "local" {
				t.Errorf("backend.Type() = %q, want %q", backend.Type(), "local")
			}
		})
	}
}

func TestBackend_Type(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-type-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	if got := backend.Type(); got != "local" {
		t.Errorf("Type() = %q, want %q", got, "local")
	}
}

func TestBackend_BasePath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-basepath-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	if got := backend.BasePath(); got != tempDir {
		t.Errorf("BasePath() = %q, want %q", got, tempDir)
	}
}

func TestBackend_MetadataExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-metadata-exists-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// MetadataExists should always return true for local storage
	// since we discover databases dynamically
	exists, err := backend.MetadataExists(context.Background())
	if err != nil {
		t.Fatalf("MetadataExists() error = %v", err)
	}
	if !exists {
		t.Error("MetadataExists() = false, want true")
	}
}

func TestBackend_ListMetadata_EmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-list-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(context.Background())
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 0 {
		t.Errorf("ListMetadata() returned %d items, want 0", len(metadata))
	}
}

func TestBackend_ListMetadata_WithDatabases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-list-dbs-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock CodeQL database
	dbDir := filepath.Join(tempDir, "test-db")
	if err := os.MkdirAll(filepath.Join(dbDir, "db-go"), 0o755); err != nil {
		t.Fatalf("Failed to create db directory: %v", err)
	}
	yamlContent := `sourceLocationPrefix: /Users/test/src/owner/repo
primaryLanguage: go
creationMetadata:
  sha: abc123
  cliVersion: 2.15.0
  creationTime: "2024-01-15T10:30:00Z"
`
	if err := os.WriteFile(filepath.Join(dbDir, "codeql-database.yml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write codeql-database.yml: %v", err)
	}

	backend, err := New(Config{
		BasePath:    tempDir,
		EndpointURL: "http://localhost:8080",
	})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(context.Background())
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 1 {
		t.Fatalf("ListMetadata() returned %d items, want 1", len(metadata))
	}

	m := metadata[0]
	if m.PrimaryLanguage != "go" {
		t.Errorf("PrimaryLanguage = %q, want %q", m.PrimaryLanguage, "go")
	}
	if m.GitOwner != "owner" {
		t.Errorf("GitOwner = %q, want %q", m.GitOwner, "owner")
	}
	if m.GitRepo != "repo" {
		t.Errorf("GitRepo = %q, want %q", m.GitRepo, "repo")
	}
	if m.Projname != "owner/repo" {
		t.Errorf("Projname = %q, want %q", m.Projname, "owner/repo")
	}
	if m.ToolVersion != "2.15.0" {
		t.Errorf("ToolVersion = %q, want %q", m.ToolVersion, "2.15.0")
	}
	if m.GitCommitID != "abc123" {
		t.Errorf("GitCommitID = %q, want %q", m.GitCommitID, "abc123")
	}
	if !strings.HasPrefix(m.ResultURL, "http://localhost:8080/db/") {
		t.Errorf("ResultURL = %q, want prefix %q", m.ResultURL, "http://localhost:8080/db/")
	}
	if m.ToolName != "codeql-go" {
		t.Errorf("ToolName = %q, want %q", m.ToolName, "codeql-go")
	}
	if m.ContentHash == "" {
		t.Error("ContentHash is empty")
	}
	if m.BuildCID == "" {
		t.Error("BuildCID is empty")
	}
}

func TestBackend_ListMetadata_Caching(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-cache-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock CodeQL database
	dbDir := filepath.Join(tempDir, "test-db")
	if err := os.MkdirAll(filepath.Join(dbDir, "db-go"), 0o755); err != nil {
		t.Fatalf("Failed to create db directory: %v", err)
	}
	yamlContent := `sourceLocationPrefix: /src/owner/repo
primaryLanguage: go
`
	if err := os.WriteFile(filepath.Join(dbDir, "codeql-database.yml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write codeql-database.yml: %v", err)
	}

	backend, err := New(Config{
		BasePath: tempDir,
		CacheTTL: 1 * time.Hour, // Long cache to ensure we hit it
	})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// First call - should discover
	metadata1, err := backend.ListMetadata(context.Background())
	if err != nil {
		t.Fatalf("First ListMetadata() error = %v", err)
	}
	if len(metadata1) != 1 {
		t.Fatalf("First ListMetadata() returned %d items, want 1", len(metadata1))
	}

	// Second call - should hit cache
	metadata2, err := backend.ListMetadata(context.Background())
	if err != nil {
		t.Fatalf("Second ListMetadata() error = %v", err)
	}
	if len(metadata2) != 1 {
		t.Fatalf("Second ListMetadata() returned %d items, want 1", len(metadata2))
	}

	// Verify same data (cached)
	if metadata1[0].ContentHash != metadata2[0].ContentHash {
		t.Error("Cache returned different data")
	}
}

func TestBackend_InvalidateCache(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-invalidate-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a database first
	dbDir := filepath.Join(tempDir, "test-db")
	if err := os.MkdirAll(filepath.Join(dbDir, "db-go"), 0o755); err != nil {
		t.Fatalf("Failed to create db directory: %v", err)
	}
	yamlContent := `sourceLocationPrefix: /src/owner/repo
primaryLanguage: go
`
	if err := os.WriteFile(filepath.Join(dbDir, "codeql-database.yml"), []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("Failed to write codeql-database.yml: %v", err)
	}

	backend, err := New(Config{
		BasePath: tempDir,
		CacheTTL: 1 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// First call - should find 1 database
	metadata1, err := backend.ListMetadata(context.Background())
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}
	if len(metadata1) != 1 {
		t.Fatalf("First ListMetadata() returned %d items, want 1", len(metadata1))
	}

	// Add another database
	dbDir2 := filepath.Join(tempDir, "test-db2")
	if err := os.MkdirAll(filepath.Join(dbDir2, "db-python"), 0o755); err != nil {
		t.Fatalf("Failed to create db2 directory: %v", err)
	}
	yamlContent2 := `sourceLocationPrefix: /src/owner2/repo2
primaryLanguage: python
`
	if err := os.WriteFile(filepath.Join(dbDir2, "codeql-database.yml"), []byte(yamlContent2), 0o644); err != nil {
		t.Fatalf("Failed to write codeql-database.yml: %v", err)
	}

	// Without invalidation, cache returns old data (1 item)
	metadata2, _ := backend.ListMetadata(context.Background())
	if len(metadata2) != 1 {
		t.Errorf("Cache should return old data (1 item), got %d", len(metadata2))
	}

	// Invalidate and re-fetch
	backend.InvalidateCache()
	metadata3, _ := backend.ListMetadata(context.Background())
	if len(metadata3) != 2 {
		t.Errorf("After invalidation, should return 2 items, got %d", len(metadata3))
	}
}

func TestBackend_GetFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-getfile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testContent := []byte("test file content")
	testFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(testFile, testContent, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	reader, size, contentType, err := backend.GetFile(context.Background(), "test.txt")
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	defer reader.Close()

	if size != int64(len(testContent)) {
		t.Errorf("size = %d, want %d", size, len(testContent))
	}

	if contentType != "text/plain; charset=utf-8" {
		t.Errorf("contentType = %q, want %q", contentType, "text/plain; charset=utf-8")
	}

	// Read content
	buf := make([]byte, size)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read file content: %v", err)
	}
	if !bytes.Equal(buf[:n], testContent) {
		t.Errorf("content = %q, want %q", string(buf[:n]), string(testContent))
	}
}

func TestBackend_GetFile_NotFound(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-getfile-notfound-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	_, _, _, err = backend.GetFile(context.Background(), "nonexistent.txt")
	if err == nil {
		t.Fatal("GetFile() expected error for non-existent file, got nil")
	}

	var notFound *storage.ErrNotFound
	if !strings.Contains(err.Error(), "not found") {
		// Check if it's an ErrNotFound
		if ok := strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "ErrNotFound"); !ok {
			t.Errorf("GetFile() error = %v, want ErrNotFound", err)
		}
	}
	_ = notFound // suppress unused warning
}

func TestBackend_GetFile_DirectoryTraversal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-traversal-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Try to access file outside base path
	_, _, _, err = backend.GetFile(context.Background(), "../../../etc/passwd")
	if err == nil {
		t.Fatal("GetFile() expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("GetFile() error = %v, want error containing 'access denied'", err)
	}
}

func TestBackend_GetFile_Directory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-getfile-dir-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a subdirectory
	subDir := filepath.Join(tempDir, "subdir")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("Failed to create subdirectory: %v", err)
	}

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	_, _, _, err = backend.GetFile(context.Background(), "subdir")
	if err == nil {
		t.Fatal("GetFile() expected error for directory, got nil")
	}
	if !strings.Contains(err.Error(), "cannot serve directory") {
		t.Errorf("GetFile() error = %v, want error containing 'cannot serve directory'", err)
	}
}

func TestBackend_FileExists(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-fileexists-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file
	testFile := filepath.Join(tempDir, "exists.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "existing file",
			filename: "exists.txt",
			want:     true,
		},
		{
			name:     "non-existent file",
			filename: "notexists.txt",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := backend.FileExists(context.Background(), tt.filename)
			if err != nil {
				t.Fatalf("FileExists() error = %v", err)
			}
			if exists != tt.want {
				t.Errorf("FileExists() = %v, want %v", exists, tt.want)
			}
		})
	}
}

func TestBackend_FileExists_DirectoryTraversal(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-fileexists-traversal-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	// Try to check file outside base path
	_, err = backend.FileExists(context.Background(), "../../../etc/passwd")
	if err == nil {
		t.Fatal("FileExists() expected error for path traversal, got nil")
	}
	if !strings.Contains(err.Error(), "access denied") {
		t.Errorf("FileExists() error = %v, want error containing 'access denied'", err)
	}
}

func TestBackend_Close(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-close-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{BasePath: tempDir})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}

	// Close should not return an error
	if err := backend.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestConvertToMetadata(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local-convert-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	backend, err := New(Config{
		BasePath:    tempDir,
		EndpointURL: "http://example.com",
	})
	if err != nil {
		t.Fatalf("Failed to create backend: %v", err)
	}
	defer backend.Close()

	tests := []struct {
		name string
		db   *codeql.DiscoveredDatabase
		want api.DatabaseMetadata
	}{
		{
			name: "with creation metadata",
			db: &codeql.DiscoveredDatabase{
				Path:                 filepath.Join(tempDir, "test-db"),
				Name:                 "test-db",
				IsArchived:           false,
				Language:             "go",
				SourceLocationPrefix: "/src/owner/repo",
				CreationMetadata: &codeql.DatabaseCreationMetadata{
					SHA:          "abc123",
					CLIVersion:   "2.15.0",
					CreationTime: "2024-01-15T10:30:00Z",
				},
				FileSize:    1000,
				ContentHash: "deadbeef",
				Owner:       "owner",
				Repo:        "repo",
			},
			want: api.DatabaseMetadata{
				ContentHash:          "deadbeef",
				GitBranch:            "HEAD",
				GitCommitID:          "abc123",
				GitOwner:             "owner",
				GitRepo:              "repo",
				IngestionDatetimeUTC: "2024-01-15T10:30:00Z",
				PrimaryLanguage:      "go",
				ToolName:             "codeql-go",
				ToolVersion:          "2.15.0",
				Projname:             "owner/repo",
				DBFileSize:           1000,
			},
		},
		{
			name: "without creation metadata",
			db: &codeql.DiscoveredDatabase{
				Path:       filepath.Join(tempDir, "simple-db"),
				Name:       "simple-db",
				IsArchived: false,
				Language:   "python",
				FileSize:   500,
				Owner:      "testowner",
				Repo:       "testrepo",
			},
			want: api.DatabaseMetadata{
				GitBranch:       "HEAD",
				GitOwner:        "testowner",
				GitRepo:         "testrepo",
				PrimaryLanguage: "python",
				ToolName:        "codeql-python",
				Projname:        "testowner/testrepo",
				DBFileSize:      500,
			},
		},
		{
			name: "unknown language",
			db: &codeql.DiscoveredDatabase{
				Path:     filepath.Join(tempDir, "unknown-db"),
				Name:     "unknown-db",
				Language: "unknown",
				Owner:    "owner",
				Repo:     "repo",
			},
			want: api.DatabaseMetadata{
				GitBranch:       "HEAD",
				GitOwner:        "owner",
				GitRepo:         "repo",
				PrimaryLanguage: "unknown",
				ToolName:        "codeql",
				Projname:        "owner/repo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := backend.convertToMetadata(tt.db)

			// Check specific fields (not all, as some are generated)
			if got.PrimaryLanguage != tt.want.PrimaryLanguage {
				t.Errorf("PrimaryLanguage = %q, want %q", got.PrimaryLanguage, tt.want.PrimaryLanguage)
			}
			if got.GitOwner != tt.want.GitOwner {
				t.Errorf("GitOwner = %q, want %q", got.GitOwner, tt.want.GitOwner)
			}
			if got.GitRepo != tt.want.GitRepo {
				t.Errorf("GitRepo = %q, want %q", got.GitRepo, tt.want.GitRepo)
			}
			if got.Projname != tt.want.Projname {
				t.Errorf("Projname = %q, want %q", got.Projname, tt.want.Projname)
			}
			if got.ToolName != tt.want.ToolName {
				t.Errorf("ToolName = %q, want %q", got.ToolName, tt.want.ToolName)
			}
			if got.GitBranch != tt.want.GitBranch {
				t.Errorf("GitBranch = %q, want %q", got.GitBranch, tt.want.GitBranch)
			}
			if got.DBFileSize != tt.want.DBFileSize {
				t.Errorf("DBFileSize = %d, want %d", got.DBFileSize, tt.want.DBFileSize)
			}

			// Check that generated fields are present
			if got.ContentHash == "" {
				t.Error("ContentHash is empty")
			}
			if got.BuildCID == "" {
				t.Error("BuildCID is empty")
			}
			if !strings.HasPrefix(got.ResultURL, "http://example.com/db/") {
				t.Errorf("ResultURL = %q, want prefix %q", got.ResultURL, "http://example.com/db/")
			}
		})
	}
}

func TestGenerateBuildCID(t *testing.T) {
	// Test that generateBuildCID produces consistent results
	cid1 := generateBuildCID("2.15.0", "2024-01-15T10:30:00Z", "go", "abc123")
	cid2 := generateBuildCID("2.15.0", "2024-01-15T10:30:00Z", "go", "abc123")

	if cid1 != cid2 {
		t.Errorf("generateBuildCID() produced inconsistent results: %q vs %q", cid1, cid2)
	}

	if len(cid1) != 10 {
		t.Errorf("generateBuildCID() length = %d, want 10", len(cid1))
	}

	// Different inputs should produce different CIDs
	cid3 := generateBuildCID("2.14.0", "2024-01-15T10:30:00Z", "go", "abc123")
	if cid1 == cid3 {
		t.Error("generateBuildCID() should produce different CIDs for different inputs")
	}
}
