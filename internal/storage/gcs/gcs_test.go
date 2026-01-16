// Package gcs tests for the Google Cloud Storage backend using fake-gcs-server.
package gcs

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/fsouza/fake-gcs-server/fakestorage"
)

// testServer wraps a fakestorage.Server and provides helper methods.
type testServer struct {
	*fakestorage.Server
	bucket string
}

// newTestServer creates a new fake GCS server with an in-memory backend.
func newTestServer(t *testing.T, bucket string) *testServer {
	t.Helper()

	server, err := fakestorage.NewServerWithOptions(fakestorage.Options{
		NoListener: true, // Use in-memory transport, no TCP listener
	})
	if err != nil {
		t.Fatalf("failed to create fake GCS server: %v", err)
	}

	server.CreateBucketWithOpts(fakestorage.CreateBucketOpts{Name: bucket})

	return &testServer{
		Server: server,
		bucket: bucket,
	}
}

// createDatabase creates a simulated CodeQL database in the fake GCS bucket.
func (s *testServer) createDatabase(t *testing.T, dbPath string, yamlContent []byte, language string) {
	t.Helper()

	// Create codeql-database.yml
	s.CreateObject(fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName:  s.bucket,
			Name:        dbPath + "/codeql-database.yml",
			ContentType: "application/x-yaml",
		},
		Content: yamlContent,
	})

	// Create db-<lang> directory marker
	if language != "" {
		s.CreateObject(fakestorage.Object{
			ObjectAttrs: fakestorage.ObjectAttrs{
				BucketName:  s.bucket,
				Name:        dbPath + "/db-" + language + "/.marker",
				ContentType: "text/plain",
			},
			Content: []byte("marker"),
		})
	}
}

// createFile creates a file object in the fake GCS bucket.
func (s *testServer) createFile(t *testing.T, path string, content []byte, contentType string) {
	t.Helper()

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	s.CreateObject(fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName:  s.bucket,
			Name:        path,
			ContentType: contentType,
		},
		Content: content,
	})
}

func TestNew(t *testing.T) {
	ctx := context.Background()

	t.Run("missing bucket", func(t *testing.T) {
		_, err := New(ctx, Config{})
		if err == nil {
			t.Error("expected error for missing bucket")
		}
		if !strings.Contains(err.Error(), "bucket name is required") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("with injected client", func(t *testing.T) {
		server := newTestServer(t, "test-bucket")
		defer server.Stop()

		backend, err := New(ctx, Config{
			Bucket:      "test-bucket",
			Client:      server.Client(),
			EndpointURL: "http://localhost:8080",
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		if backend.Type() != "gcs" {
			t.Errorf("Type() = %q, want %q", backend.Type(), "gcs")
		}
	})

	t.Run("with prefix", func(t *testing.T) {
		server := newTestServer(t, "test-bucket")
		defer server.Stop()

		backend, err := New(ctx, Config{
			Bucket:      "test-bucket",
			Client:      server.Client(),
			Prefix:      "databases", // without trailing slash
			EndpointURL: "http://localhost:8080",
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		// The prefix should have a trailing slash added
		if backend.prefix != "databases/" {
			t.Errorf("prefix = %q, want %q", backend.prefix, "databases/")
		}
	})

	t.Run("with prefix with trailing slash", func(t *testing.T) {
		server := newTestServer(t, "test-bucket")
		defer server.Stop()

		backend, err := New(ctx, Config{
			Bucket:      "test-bucket",
			Client:      server.Client(),
			Prefix:      "databases/",
			EndpointURL: "http://localhost:8080",
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		if backend.prefix != "databases/" {
			t.Errorf("prefix = %q, want %q", backend.prefix, "databases/")
		}
	})

	t.Run("default endpoint URL", func(t *testing.T) {
		server := newTestServer(t, "test-bucket")
		defer server.Stop()

		backend, err := New(ctx, Config{
			Bucket: "test-bucket",
			Client: server.Client(),
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		if backend.endpointURL != "http://localhost:8080" {
			t.Errorf("endpointURL = %q, want %q", backend.endpointURL, "http://localhost:8080")
		}
	})

	t.Run("default cache TTL", func(t *testing.T) {
		server := newTestServer(t, "test-bucket")
		defer server.Stop()

		backend, err := New(ctx, Config{
			Bucket: "test-bucket",
			Client: server.Client(),
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		if backend.cacheTTL != 5*time.Minute {
			t.Errorf("cacheTTL = %v, want %v", backend.cacheTTL, 5*time.Minute)
		}
	})

	t.Run("custom cache TTL", func(t *testing.T) {
		server := newTestServer(t, "test-bucket")
		defer server.Stop()

		backend, err := New(ctx, Config{
			Bucket:   "test-bucket",
			Client:   server.Client(),
			CacheTTL: 10 * time.Minute,
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		if backend.cacheTTL != 10*time.Minute {
			t.Errorf("cacheTTL = %v, want %v", backend.cacheTTL, 10*time.Minute)
		}
	})
}

func TestBackend_Type(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	if got := backend.Type(); got != "gcs" {
		t.Errorf("Type() = %q, want %q", got, "gcs")
	}
}

func TestBackend_MetadataExists(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	// MetadataExists always returns true for GCS (dynamic discovery)
	exists, err := backend.MetadataExists(ctx)
	if err != nil {
		t.Errorf("MetadataExists() error = %v", err)
	}
	if !exists {
		t.Error("MetadataExists() = false, want true")
	}
}

func TestBackend_ListMetadata_EmptyBucket(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 0 {
		t.Errorf("ListMetadata() returned %d items, want 0", len(metadata))
	}
}

func TestBackend_ListMetadata_WithDatabases(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create a database with full metadata
	yamlContent := []byte(`
sourceLocationPrefix: "/home/user/github/testowner/testrepo"
primaryLanguage: go
creationMetadata:
  sha: abc123def456
  cliVersion: "2.15.0"
  creationTime: "2024-01-15T10:30:00Z"
`)
	server.createDatabase(t, "testrepo-db", yamlContent, "go")

	backend, err := New(ctx, Config{
		Bucket:      "test-bucket",
		Client:      server.Client(),
		EndpointURL: "http://example.com",
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 1 {
		t.Fatalf("ListMetadata() returned %d items, want 1", len(metadata))
	}

	m := metadata[0]

	// Check metadata fields
	if m.GitOwner != "testowner" {
		t.Errorf("GitOwner = %q, want %q", m.GitOwner, "testowner")
	}
	if m.GitRepo != "testrepo" {
		t.Errorf("GitRepo = %q, want %q", m.GitRepo, "testrepo")
	}
	if m.PrimaryLanguage != "go" {
		t.Errorf("PrimaryLanguage = %q, want %q", m.PrimaryLanguage, "go")
	}
	if m.ToolVersion != "2.15.0" {
		t.Errorf("ToolVersion = %q, want %q", m.ToolVersion, "2.15.0")
	}
	if m.GitCommitID != "abc123def456" {
		t.Errorf("GitCommitID = %q, want %q", m.GitCommitID, "abc123def456")
	}
	if m.IngestionDatetimeUTC != "2024-01-15T10:30:00Z" {
		t.Errorf("IngestionDatetimeUTC = %q, want %q", m.IngestionDatetimeUTC, "2024-01-15T10:30:00Z")
	}
	if m.Projname != "testowner/testrepo" {
		t.Errorf("Projname = %q, want %q", m.Projname, "testowner/testrepo")
	}
	if !strings.HasPrefix(m.ResultURL, "http://example.com/db/") {
		t.Errorf("ResultURL = %q, want prefix %q", m.ResultURL, "http://example.com/db/")
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

func TestBackend_ListMetadata_MultipleDatabases(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create multiple databases
	goYAML := []byte(`
sourceLocationPrefix: "/home/user/github/owner1/go-project"
primaryLanguage: go
`)
	server.createDatabase(t, "go-project-db", goYAML, "go")

	pythonYAML := []byte(`
sourceLocationPrefix: "/home/user/github/owner2/python-project"
primaryLanguage: python
`)
	server.createDatabase(t, "python-project-db", pythonYAML, "python")

	jsYAML := []byte(`
sourceLocationPrefix: "/home/user/github/owner3/js-project"
primaryLanguage: javascript
`)
	server.createDatabase(t, "js-project-db", jsYAML, "javascript")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 3 {
		t.Errorf("ListMetadata() returned %d items, want 3", len(metadata))
	}

	// Verify we have all languages
	languages := make(map[string]bool)
	for _, m := range metadata {
		languages[m.PrimaryLanguage] = true
	}

	expectedLangs := []string{"go", "python", "javascript"}
	for _, lang := range expectedLangs {
		if !languages[lang] {
			t.Errorf("missing language %q in results", lang)
		}
	}
}

func TestBackend_ListMetadata_WithPrefix(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create database in a subdirectory
	yamlContent := []byte(`
sourceLocationPrefix: "/home/user/github/testowner/testrepo"
primaryLanguage: go
`)
	server.createDatabase(t, "databases/testrepo-db", yamlContent, "go")

	// Also create a database outside the prefix (should be ignored)
	otherYAML := []byte(`
sourceLocationPrefix: "/home/user/github/other/other"
primaryLanguage: python
`)
	server.createDatabase(t, "other/other-db", otherYAML, "python")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
		Prefix: "databases/",
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 1 {
		t.Errorf("ListMetadata() returned %d items, want 1", len(metadata))
	}

	if len(metadata) > 0 && metadata[0].PrimaryLanguage != "go" {
		t.Errorf("expected go database, got %q", metadata[0].PrimaryLanguage)
	}
}

func TestBackend_ListMetadata_Caching(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create initial database
	yamlContent := []byte(`
sourceLocationPrefix: "/home/user/github/testowner/testrepo"
primaryLanguage: go
`)
	server.createDatabase(t, "testrepo-db", yamlContent, "go")

	backend, err := New(ctx, Config{
		Bucket:   "test-bucket",
		Client:   server.Client(),
		CacheTTL: 1 * time.Hour, // Long TTL for test
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	// First call populates cache
	metadata1, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata1) != 1 {
		t.Fatalf("first ListMetadata() returned %d items, want 1", len(metadata1))
	}

	// Add another database (won't be seen due to cache)
	pythonYAML := []byte(`
sourceLocationPrefix: "/home/user/github/other/python-project"
primaryLanguage: python
`)
	server.createDatabase(t, "python-project-db", pythonYAML, "python")

	// Second call returns cached data
	metadata2, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	// Should still be 1 (cached)
	if len(metadata2) != 1 {
		t.Errorf("second ListMetadata() returned %d items, want 1 (cached)", len(metadata2))
	}
}

func TestBackend_ListMetadata_LanguageDetection(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create database without primaryLanguage in YAML - should detect from db-<lang> directory
	yamlContent := []byte(`
sourceLocationPrefix: "/home/user/github/testowner/testrepo"
`)
	server.createDatabase(t, "testrepo-db", yamlContent, "python")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 1 {
		t.Fatalf("ListMetadata() returned %d items, want 1", len(metadata))
	}

	// Language should be detected from db-python directory
	if metadata[0].PrimaryLanguage != "python" {
		t.Errorf("PrimaryLanguage = %q, want %q (detected from directory)", metadata[0].PrimaryLanguage, "python")
	}
}

func TestBackend_GetFile(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create a test file
	testContent := []byte("Hello, GCS!")
	server.createFile(t, "test-file.txt", testContent, "text/plain")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	reader, size, contentType, err := backend.GetFile(ctx, "test-file.txt")
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	defer reader.Close()

	if size != int64(len(testContent)) {
		t.Errorf("GetFile() size = %d, want %d", size, len(testContent))
	}

	if contentType != "text/plain" {
		t.Errorf("GetFile() contentType = %q, want %q", contentType, "text/plain")
	}

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(content, testContent) {
		t.Errorf("GetFile() content = %q, want %q", string(content), string(testContent))
	}
}

func TestBackend_GetFile_WithPrefix(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create a file under a prefix
	testContent := []byte("Prefixed content")
	server.createFile(t, "databases/test-file.txt", testContent, "text/plain")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
		Prefix: "databases/",
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	// Request without prefix - backend should add it
	reader, _, _, err := backend.GetFile(ctx, "test-file.txt")
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	defer reader.Close()

	content, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if !bytes.Equal(content, testContent) {
		t.Errorf("GetFile() content = %q, want %q", string(content), string(testContent))
	}
}

func TestBackend_GetFile_NotFound(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	_, _, _, err = backend.GetFile(ctx, "nonexistent-file.txt")
	if err == nil {
		t.Error("GetFile() expected error for nonexistent file")
	}
}

func TestBackend_GetFile_DefaultContentType(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create a file with empty content type
	server.CreateObject(fakestorage.Object{
		ObjectAttrs: fakestorage.ObjectAttrs{
			BucketName: "test-bucket",
			Name:       "binary-file.bin",
			// ContentType intentionally empty
		},
		Content: []byte{0x00, 0x01, 0x02},
	})

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	reader, _, contentType, err := backend.GetFile(ctx, "binary-file.bin")
	if err != nil {
		t.Fatalf("GetFile() error = %v", err)
	}
	defer reader.Close()

	// Should default to application/octet-stream
	if contentType != "application/octet-stream" {
		t.Errorf("GetFile() contentType = %q, want %q", contentType, "application/octet-stream")
	}
}

func TestBackend_FileExists(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	server.createFile(t, "existing-file.txt", []byte("content"), "text/plain")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	t.Run("existing file", func(t *testing.T) {
		exists, err := backend.FileExists(ctx, "existing-file.txt")
		if err != nil {
			t.Errorf("FileExists() error = %v", err)
		}
		if !exists {
			t.Error("FileExists() = false, want true")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		exists, err := backend.FileExists(ctx, "nonexistent-file.txt")
		if err != nil {
			t.Errorf("FileExists() error = %v", err)
		}
		if exists {
			t.Error("FileExists() = true, want false")
		}
	})
}

func TestBackend_FileExists_WithPrefix(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	server.createFile(t, "databases/existing-file.txt", []byte("content"), "text/plain")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
		Prefix: "databases/",
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	// Check without prefix - backend adds it
	exists, err := backend.FileExists(ctx, "existing-file.txt")
	if err != nil {
		t.Errorf("FileExists() error = %v", err)
	}
	if !exists {
		t.Error("FileExists() = false, want true")
	}
}

func TestBackend_Close(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}

	// Pre-populate cache
	_, _ = backend.ListMetadata(ctx) //nolint:errcheck // ignoring error for cache pre-population in test

	// Close should clear cache and close client
	err = backend.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	// Verify cache is cleared
	backend.mu.RLock()
	if backend.cachedMetadata != nil {
		t.Error("cache should be cleared after Close()")
	}
	backend.mu.RUnlock()
}

func TestExtractOwnerRepo(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
	}{
		{
			name:      "full path",
			input:     "/home/user/github/myowner/myrepo",
			wantOwner: "myowner",
			wantRepo:  "myrepo",
		},
		{
			name:      "short path",
			input:     "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
		{
			name:      "single component",
			input:     "repo",
			wantOwner: "unknown",
			wantRepo:  "repo",
		},
		{
			name:      "empty",
			input:     "",
			wantOwner: "unknown",
			wantRepo:  "unknown",
		},
		{
			name:      "root path",
			input:     "/",
			wantOwner: "unknown",
			wantRepo:  "unknown",
		},
		{
			name:      "deep path",
			input:     "/a/b/c/d/e/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, gotRepo := extractOwnerRepo(tt.input)
			if gotOwner != tt.wantOwner {
				t.Errorf("extractOwnerRepo(%q) owner = %q, want %q", tt.input, gotOwner, tt.wantOwner)
			}
			if gotRepo != tt.wantRepo {
				t.Errorf("extractOwnerRepo(%q) repo = %q, want %q", tt.input, gotRepo, tt.wantRepo)
			}
		})
	}
}

func TestGenerateBuildCID(t *testing.T) {
	cid := generateBuildCID("2.15.0", "2024-01-15T10:30:00Z", "go", "abc123")

	// BuildCID should be 10 characters (truncated SHA256)
	if len(cid) != 10 {
		t.Errorf("generateBuildCID() length = %d, want 10", len(cid))
	}

	// Same inputs should produce same output
	cid2 := generateBuildCID("2.15.0", "2024-01-15T10:30:00Z", "go", "abc123")
	if cid != cid2 {
		t.Errorf("generateBuildCID() not deterministic: %q != %q", cid, cid2)
	}

	// Different inputs should produce different output
	cid3 := generateBuildCID("2.16.0", "2024-01-15T10:30:00Z", "go", "abc123")
	if cid == cid3 {
		t.Errorf("generateBuildCID() should produce different CID for different inputs")
	}
}

func TestBackend_ListMetadata_WithoutCreationMetadata(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	// Create database without creationMetadata
	yamlContent := []byte(`
sourceLocationPrefix: "/home/user/github/testowner/testrepo"
primaryLanguage: python
`)
	server.createDatabase(t, "testrepo-db", yamlContent, "python")

	backend, err := New(ctx, Config{
		Bucket: "test-bucket",
		Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("failed to create backend: %v", err)
	}
	defer backend.Close()

	metadata, err := backend.ListMetadata(ctx)
	if err != nil {
		t.Fatalf("ListMetadata() error = %v", err)
	}

	if len(metadata) != 1 {
		t.Fatalf("ListMetadata() returned %d items, want 1", len(metadata))
	}

	m := metadata[0]

	// BuildCID should still be generated (truncated content hash)
	if len(m.BuildCID) != 10 {
		t.Errorf("BuildCID length = %d, want 10", len(m.BuildCID))
	}

	// These fields should be empty without creationMetadata
	if m.ToolVersion != "" {
		t.Errorf("ToolVersion = %q, want empty", m.ToolVersion)
	}
	if m.GitCommitID != "" {
		t.Errorf("GitCommitID = %q, want empty", m.GitCommitID)
	}
	if m.IngestionDatetimeUTC != "" {
		t.Errorf("IngestionDatetimeUTC = %q, want empty", m.IngestionDatetimeUTC)
	}
}

func TestBackend_objectPath(t *testing.T) {
	ctx := context.Background()
	server := newTestServer(t, "test-bucket")
	defer server.Stop()

	t.Run("without prefix", func(t *testing.T) {
		backend, err := New(ctx, Config{
			Bucket: "test-bucket",
			Client: server.Client(),
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		path := backend.objectPath("test.txt")
		if path != "test.txt" {
			t.Errorf("objectPath() = %q, want %q", path, "test.txt")
		}
	})

	t.Run("with prefix", func(t *testing.T) {
		backend, err := New(ctx, Config{
			Bucket: "test-bucket",
			Client: server.Client(),
			Prefix: "databases/",
		})
		if err != nil {
			t.Fatalf("failed to create backend: %v", err)
		}
		defer backend.Close()

		path := backend.objectPath("test.txt")
		if path != "databases/test.txt" {
			t.Errorf("objectPath() = %q, want %q", path, "databases/test.txt")
		}
	})
}
