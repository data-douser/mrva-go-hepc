// Package server provides the HTTP server implementation for MRVA HEPC.
package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/data-douser/mrva-go-hepc/api"
	"github.com/data-douser/mrva-go-hepc/internal/storage"
)

// mockBackend implements storage.Backend for testing
type mockBackend struct {
	typeStr        string
	metadata       []api.DatabaseMetadata
	metadataError  error
	metadataExists bool
	existsError    error
	fileContent    string
	fileSize       int64
	fileType       string
	fileError      error
	fileExists     bool
	fileExistsErr  error
}

func (m *mockBackend) Type() string {
	return m.typeStr
}

func (m *mockBackend) ListMetadata(ctx context.Context) ([]api.DatabaseMetadata, error) {
	if m.metadataError != nil {
		return nil, m.metadataError
	}
	return m.metadata, nil
}

func (m *mockBackend) GetFile(ctx context.Context, filename string) (io.ReadCloser, int64, string, error) {
	if m.fileError != nil {
		return nil, 0, "", m.fileError
	}
	reader := io.NopCloser(strings.NewReader(m.fileContent))
	return reader, m.fileSize, m.fileType, nil
}

func (m *mockBackend) FileExists(ctx context.Context, filename string) (bool, error) {
	return m.fileExists, m.fileExistsErr
}

func (m *mockBackend) MetadataExists(ctx context.Context) (bool, error) {
	if m.existsError != nil {
		return false, m.existsError
	}
	return m.metadataExists, nil
}

func (m *mockBackend) Close() error {
	return nil
}

func TestNew(t *testing.T) {
	mock := &mockBackend{typeStr: "mock"}
	cfg := Config{Host: "127.0.0.1", Port: 8080}

	srv := New(cfg, mock, nil)
	if srv == nil {
		t.Fatal("New() returned nil")
	}
}

func TestServer_Handler(t *testing.T) {
	mock := &mockBackend{typeStr: "mock"}
	cfg := Config{Host: "127.0.0.1", Port: 8080}
	srv := New(cfg, mock, slog.Default())

	handler := srv.Handler()
	if handler == nil {
		t.Fatal("Handler() returned nil")
	}
}

func TestServer_handleHealth(t *testing.T) {
	tests := []struct {
		name           string
		backend        *mockBackend
		expectedStatus int
		checkBody      func(t *testing.T, body string)
	}{
		{
			name: "healthy with metadata",
			backend: &mockBackend{
				typeStr:        "local",
				metadataExists: true,
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp struct {
					Status        string `json:"status"`
					StorageType   string `json:"storage_type"`
					HasMetadataDB bool   `json:"has_metadata_db"`
				}
				if err := json.Unmarshal([]byte(body), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.Status != "ok" {
					t.Errorf("status = %q, want %q", resp.Status, "ok")
				}
				if resp.StorageType != "local" {
					t.Errorf("storage_type = %q, want %q", resp.StorageType, "local")
				}
				if !resp.HasMetadataDB {
					t.Error("has_metadata_db = false, want true")
				}
			},
		},
		{
			name: "healthy without metadata",
			backend: &mockBackend{
				typeStr:        "gcs",
				metadataExists: false,
			},
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				var resp struct {
					HasMetadataDB bool `json:"has_metadata_db"`
				}
				if err := json.Unmarshal([]byte(body), &resp); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if resp.HasMetadataDB {
					t.Error("has_metadata_db = true, want false")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := New(Config{}, tt.backend, slog.Default())
			req := httptest.NewRequest("GET", "/health", http.NoBody)
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

func TestServer_handleMetadata(t *testing.T) {
	tests := []struct {
		name           string
		backend        *mockBackend
		path           string
		expectedStatus int
		checkBody      func(t *testing.T, body string)
	}{
		{
			name: "index endpoint with data",
			backend: &mockBackend{
				typeStr:        "local",
				metadataExists: true,
				metadata: []api.DatabaseMetadata{
					{
						ContentHash:     "abc123",
						GitOwner:        "owner",
						GitRepo:         "repo",
						PrimaryLanguage: "go",
						Projname:        "owner/repo",
					},
				},
			},
			path:           "/index",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "owner/repo") {
					t.Errorf("body should contain 'owner/repo', got %q", body)
				}
				if !strings.Contains(body, "abc123") {
					t.Errorf("body should contain 'abc123', got %q", body)
				}
			},
		},
		{
			name: "api endpoint with data",
			backend: &mockBackend{
				typeStr:        "local",
				metadataExists: true,
				metadata: []api.DatabaseMetadata{
					{
						ContentHash:     "def456",
						GitOwner:        "owner2",
						GitRepo:         "repo2",
						PrimaryLanguage: "python",
						Projname:        "owner2/repo2",
					},
				},
			},
			path:           "/api/v1/latest_results/codeql-all",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				if !strings.Contains(body, "owner2/repo2") {
					t.Errorf("body should contain 'owner2/repo2', got %q", body)
				}
			},
		},
		{
			name: "empty metadata",
			backend: &mockBackend{
				typeStr:        "local",
				metadataExists: true,
				metadata:       []api.DatabaseMetadata{},
			},
			path:           "/index",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				if body != "" {
					t.Errorf("body should be empty, got %q", body)
				}
			},
		},
		{
			name: "metadata not found",
			backend: &mockBackend{
				typeStr:        "local",
				metadataExists: false,
			},
			path:           "/index",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "metadata error",
			backend: &mockBackend{
				typeStr:        "local",
				metadataExists: true,
				metadataError:  &mockError{"database error"},
			},
			path:           "/index",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "multiple metadata entries",
			backend: &mockBackend{
				typeStr:        "local",
				metadataExists: true,
				metadata: []api.DatabaseMetadata{
					{ContentHash: "hash1", Projname: "owner1/repo1"},
					{ContentHash: "hash2", Projname: "owner2/repo2"},
					{ContentHash: "hash3", Projname: "owner3/repo3"},
				},
			},
			path:           "/index",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				// JSONL format - entries separated by newlines
				lines := strings.Split(strings.TrimSpace(body), "\n")
				if len(lines) != 3 {
					t.Errorf("expected 3 lines, got %d: %q", len(lines), body)
				}
				for _, line := range lines {
					var m api.DatabaseMetadata
					if err := json.Unmarshal([]byte(line), &m); err != nil {
						t.Errorf("Failed to parse JSONL line: %v", err)
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := New(Config{}, tt.backend, slog.Default())
			req := httptest.NewRequest("GET", tt.path, http.NoBody)
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

func TestServer_handleServeFile(t *testing.T) {
	tests := []struct {
		name           string
		backend        *mockBackend
		path           string
		expectedStatus int
		checkHeaders   func(t *testing.T, h http.Header)
		checkBody      func(t *testing.T, body string)
	}{
		{
			name: "serve file successfully",
			backend: &mockBackend{
				typeStr:     "local",
				fileContent: "file content here",
				fileSize:    17,
				fileType:    "text/plain",
			},
			path:           "/db/test/file.txt",
			expectedStatus: http.StatusOK,
			checkHeaders: func(t *testing.T, h http.Header) {
				if ct := h.Get("Content-Type"); ct != "text/plain" {
					t.Errorf("Content-Type = %q, want %q", ct, "text/plain")
				}
				if cl := h.Get("Content-Length"); cl != "17" {
					t.Errorf("Content-Length = %q, want %q", cl, "17")
				}
			},
			checkBody: func(t *testing.T, body string) {
				if body != "file content here" {
					t.Errorf("body = %q, want %q", body, "file content here")
				}
			},
		},
		{
			name: "file not found",
			backend: &mockBackend{
				typeStr:   "local",
				fileError: &storage.ErrNotFound{Path: "nonexistent.txt"},
			},
			path:           "/db/nonexistent.txt",
			expectedStatus: http.StatusNotFound,
		},
		{
			name: "server error",
			backend: &mockBackend{
				typeStr:   "local",
				fileError: &mockError{"internal error"},
			},
			path:           "/db/test.txt",
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "nested path",
			backend: &mockBackend{
				typeStr:     "local",
				fileContent: "nested content",
				fileSize:    14,
				fileType:    "application/octet-stream",
			},
			path:           "/db/path/to/nested/file.zip",
			expectedStatus: http.StatusOK,
			checkBody: func(t *testing.T, body string) {
				if body != "nested content" {
					t.Errorf("body = %q, want %q", body, "nested content")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := New(Config{}, tt.backend, slog.Default())
			req := httptest.NewRequest("GET", tt.path, http.NoBody)
			w := httptest.NewRecorder()

			srv.Handler().ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.expectedStatus)
			}

			if tt.checkHeaders != nil {
				tt.checkHeaders(t, w.Header())
			}

			if tt.checkBody != nil {
				tt.checkBody(t, w.Body.String())
			}
		})
	}
}

func TestServer_handleServeFile_EmptyPath(t *testing.T) {
	backend := &mockBackend{typeStr: "local"}
	srv := New(Config{}, backend, slog.Default())

	// Test the /db endpoint without a file path
	req := httptest.NewRequest("GET", "/db/", http.NoBody)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// An empty filepath should still work due to the wildcard pattern
	// The actual behavior depends on what the mock returns
}

func TestServer_loggingMiddleware(t *testing.T) {
	backend := &mockBackend{
		typeStr:        "local",
		metadataExists: true,
		metadata:       []api.DatabaseMetadata{},
	}
	srv := New(Config{}, backend, slog.Default())

	req := httptest.NewRequest("GET", "/health", http.NoBody)
	w := httptest.NewRecorder()

	// This should not panic and should process the request
	srv.Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestServer_ContentType(t *testing.T) {
	backend := &mockBackend{
		typeStr:        "local",
		metadataExists: true,
		metadata:       []api.DatabaseMetadata{{ContentHash: "test"}},
	}
	srv := New(Config{}, backend, slog.Default())

	req := httptest.NewRequest("GET", "/index", http.NoBody)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Metadata should be served as JSONL
	ct := w.Header().Get("Content-Type")
	if ct != "application/x-ndjson" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/x-ndjson")
	}
}

func TestServer_HealthContentType(t *testing.T) {
	backend := &mockBackend{
		typeStr:        "local",
		metadataExists: true,
	}
	srv := New(Config{}, backend, slog.Default())

	req := httptest.NewRequest("GET", "/health", http.NoBody)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Health should be served as JSON
	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want %q", ct, "application/json")
	}
}

func TestServer_UnknownRoute(t *testing.T) {
	backend := &mockBackend{typeStr: "local"}
	srv := New(Config{}, backend, slog.Default())

	req := httptest.NewRequest("GET", "/unknown/route", http.NoBody)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Unknown routes should return 404
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestServer_WrongMethod(t *testing.T) {
	backend := &mockBackend{
		typeStr:        "local",
		metadataExists: true,
	}
	srv := New(Config{}, backend, slog.Default())

	// POST to GET-only endpoint
	req := httptest.NewRequest("POST", "/health", http.NoBody)
	w := httptest.NewRecorder()

	srv.Handler().ServeHTTP(w, req)

	// Should return method not allowed or not found
	if w.Code != http.StatusMethodNotAllowed && w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d or %d", w.Code, http.StatusMethodNotAllowed, http.StatusNotFound)
	}
}

// mockError is a simple error type for testing
type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}
