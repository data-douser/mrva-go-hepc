// Package storage tests for storage interface and error types.
package storage

import (
	"errors"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MetadataFile != "metadata.sql" {
		t.Errorf("DefaultConfig().MetadataFile = %q, want %q", cfg.MetadataFile, "metadata.sql")
	}
}

func TestConfig_CustomMetadataFile(t *testing.T) {
	cfg := Config{
		MetadataFile: "custom-metadata.db",
	}

	if cfg.MetadataFile != "custom-metadata.db" {
		t.Errorf("Config.MetadataFile = %q, want %q", cfg.MetadataFile, "custom-metadata.db")
	}
}

func TestErrNotFound_Error(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple path",
			path:     "file.txt",
			expected: "file not found: file.txt",
		},
		{
			name:     "path with directory",
			path:     "dir/subdir/file.txt",
			expected: "file not found: dir/subdir/file.txt",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "file not found: ",
		},
		{
			name:     "database path",
			path:     "databases/repo.zip",
			expected: "file not found: databases/repo.zip",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ErrNotFound{Path: tt.path}
			if err.Error() != tt.expected {
				t.Errorf("ErrNotFound{%q}.Error() = %q, want %q", tt.path, err.Error(), tt.expected)
			}
		})
	}
}

func TestErrNotFound_ImplementsError(t *testing.T) {
	err := &ErrNotFound{Path: "test"}
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

func TestErrMetadataNotFound_Error(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "default metadata file",
			path:     "metadata.sql",
			expected: "metadata database not found: metadata.sql",
		},
		{
			name:     "custom metadata file",
			path:     "custom/path/metadata.db",
			expected: "metadata database not found: custom/path/metadata.db",
		},
		{
			name:     "empty path",
			path:     "",
			expected: "metadata database not found: ",
		},
		{
			name:     "GCS path",
			path:     "gs://bucket/metadata.sql",
			expected: "metadata database not found: gs://bucket/metadata.sql",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ErrMetadataNotFound{Path: tt.path}
			if err.Error() != tt.expected {
				t.Errorf("ErrMetadataNotFound{%q}.Error() = %q, want %q", tt.path, err.Error(), tt.expected)
			}
		})
	}
}

func TestErrMetadataNotFound_ImplementsError(t *testing.T) {
	err := &ErrMetadataNotFound{Path: "test"}
	if err.Error() == "" {
		t.Error("Error() should not be empty")
	}
}

// TestErrorTypes_ErrorsAs verifies error types can be checked via errors.As
func TestErrorTypes_ErrorsAs(t *testing.T) {
	t.Run("ErrNotFound errors.As", func(t *testing.T) {
		var err error = &ErrNotFound{Path: "test.txt"}

		var notFound *ErrNotFound
		if !errors.As(err, &notFound) {
			t.Error("expected errors.As to find ErrNotFound")
		}
		if notFound.Path != "test.txt" {
			t.Errorf("notFound.Path = %q, want %q", notFound.Path, "test.txt")
		}
	})

	t.Run("ErrMetadataNotFound errors.As", func(t *testing.T) {
		var err error = &ErrMetadataNotFound{Path: "metadata.sql"}

		var metaErr *ErrMetadataNotFound
		if !errors.As(err, &metaErr) {
			t.Error("expected errors.As to find ErrMetadataNotFound")
		}
		if metaErr.Path != "metadata.sql" {
			t.Errorf("metaErr.Path = %q, want %q", metaErr.Path, "metadata.sql")
		}
	})
}

// TestConfig_ZeroValue ensures zero-value config has empty string
func TestConfig_ZeroValue(t *testing.T) {
	var cfg Config
	if cfg.MetadataFile != "" {
		t.Errorf("zero-value Config.MetadataFile = %q, want empty string", cfg.MetadataFile)
	}
}
