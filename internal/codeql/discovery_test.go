// Package codeql provides utilities for discovering CodeQL databases
// and extracting their metadata.
package codeql

import (
	"archive/zip"
	"encoding/xml"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestExtractOwnerRepo(t *testing.T) {
	tests := []struct {
		name                 string
		sourceLocationPrefix string
		expectedOwner        string
		expectedRepo         string
	}{
		{
			name:                 "github style path",
			sourceLocationPrefix: "/Users/xavier/src/github/titus-control-plane",
			expectedOwner:        "github",
			expectedRepo:         "titus-control-plane",
		},
		{
			name:                 "chris minio path",
			sourceLocationPrefix: "/Users/xavier/src/chris/minio",
			expectedOwner:        "chris",
			expectedRepo:         "minio",
		},
		{
			name:                 "simple two level path",
			sourceLocationPrefix: "/owner/repo",
			expectedOwner:        "owner",
			expectedRepo:         "repo",
		},
		{
			name:                 "empty path",
			sourceLocationPrefix: "",
			expectedOwner:        "unknown",
			expectedRepo:         "unknown",
		},
		{
			name:                 "single component path",
			sourceLocationPrefix: "/repo",
			expectedOwner:        "unknown",
			expectedRepo:         "repo",
		},
		{
			name:                 "deep path",
			sourceLocationPrefix: "/a/b/c/d/owner/repo",
			expectedOwner:        "owner",
			expectedRepo:         "repo",
		},
		{
			name:                 "windows style path with forward slashes",
			sourceLocationPrefix: "C:/Users/dev/src/owner/repo",
			expectedOwner:        "owner",
			expectedRepo:         "repo",
		},
		{
			name:                 "path with trailing slash",
			sourceLocationPrefix: "/owner/repo/",
			expectedOwner:        "owner",
			expectedRepo:         "repo",
		},
		{
			name:                 "just slash",
			sourceLocationPrefix: "/",
			expectedOwner:        "unknown",
			expectedRepo:         "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo := extractOwnerRepo(tt.sourceLocationPrefix)
			if owner != tt.expectedOwner {
				t.Errorf("extractOwnerRepo(%q) owner = %q, want %q", tt.sourceLocationPrefix, owner, tt.expectedOwner)
			}
			if repo != tt.expectedRepo {
				t.Errorf("extractOwnerRepo(%q) repo = %q, want %q", tt.sourceLocationPrefix, repo, tt.expectedRepo)
			}
		})
	}
}

func TestExtractOwnerRepoFromFilename(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		expectedOwner string
		expectedRepo  string
	}{
		{
			name:          "underscore separated",
			filename:      "u-boot_u-boot_cpp-srcVersion_hash.zip",
			expectedOwner: "u-boot",
			expectedRepo:  "u-boot",
		},
		{
			name:          "simple underscore",
			filename:      "owner_repo.zip",
			expectedOwner: "owner",
			expectedRepo:  "repo",
		},
		{
			name:          "dash separated",
			filename:      "owner-repo.zip",
			expectedOwner: "owner",
			expectedRepo:  "repo",
		},
		{
			name:          "tar.gz extension",
			filename:      "owner_repo.tar.gz",
			expectedOwner: "owner",
			expectedRepo:  "repo",
		},
		{
			name:          "tgz extension",
			filename:      "owner_repo.tgz",
			expectedOwner: "owner",
			expectedRepo:  "repo",
		},
		{
			name:          "no separator",
			filename:      "database.zip",
			expectedOwner: "unknown",
			expectedRepo:  "unknown",
		},
		{
			name:          "complex filename with underscores",
			filename:      "github_actions_runner_javascript.zip",
			expectedOwner: "github",
			expectedRepo:  "actions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo := extractOwnerRepoFromFilename(tt.filename)
			if owner != tt.expectedOwner {
				t.Errorf("extractOwnerRepoFromFilename(%q) owner = %q, want %q", tt.filename, owner, tt.expectedOwner)
			}
			if repo != tt.expectedRepo {
				t.Errorf("extractOwnerRepoFromFilename(%q) repo = %q, want %q", tt.filename, repo, tt.expectedRepo)
			}
		})
	}
}

func TestDetectLanguageFromDirectory(t *testing.T) {
	// Create a temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "codeql-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		setupFunc    func(baseDir string) string
		expectedLang string
	}{
		{
			name: "db-go directory",
			setupFunc: func(baseDir string) string {
				dbDir := filepath.Join(baseDir, "test-db")
				os.MkdirAll(filepath.Join(dbDir, "db-go"), 0o755)
				return dbDir
			},
			expectedLang: "go",
		},
		{
			name: "db-java directory",
			setupFunc: func(baseDir string) string {
				dbDir := filepath.Join(baseDir, "test-db-java")
				os.MkdirAll(filepath.Join(dbDir, "db-java"), 0o755)
				return dbDir
			},
			expectedLang: "java",
		},
		{
			name: "db-javascript directory",
			setupFunc: func(baseDir string) string {
				dbDir := filepath.Join(baseDir, "test-db-js")
				os.MkdirAll(filepath.Join(dbDir, "db-javascript"), 0o755)
				return dbDir
			},
			expectedLang: "javascript",
		},
		{
			name: "no db- directory",
			setupFunc: func(baseDir string) string {
				dbDir := filepath.Join(baseDir, "test-db-none")
				os.MkdirAll(filepath.Join(dbDir, "src"), 0o755)
				os.MkdirAll(filepath.Join(dbDir, "log"), 0o755)
				return dbDir
			},
			expectedLang: "unknown",
		},
		{
			name: "empty directory",
			setupFunc: func(baseDir string) string {
				dbDir := filepath.Join(baseDir, "test-db-empty")
				os.MkdirAll(dbDir, 0o755)
				return dbDir
			},
			expectedLang: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dbPath := tt.setupFunc(tempDir)
			lang := detectLanguageFromDirectory(dbPath)
			if lang != tt.expectedLang {
				t.Errorf("detectLanguageFromDirectory(%q) = %q, want %q", dbPath, lang, tt.expectedLang)
			}
		})
	}
}

func TestDetectLanguageFromZip(t *testing.T) {
	// Create a temporary directory for test zip files
	tempDir, err := os.MkdirTemp("", "codeql-zip-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	tests := []struct {
		name         string
		zipContents  []string // paths to include in the zip
		expectedLang string
	}{
		{
			name:         "db-go path in zip",
			zipContents:  []string{"db/db-go/default/", "db/db-go/default/data.db"},
			expectedLang: "go",
		},
		{
			name:         "db-python path in zip",
			zipContents:  []string{"mydb/db-python/something.txt"},
			expectedLang: "python",
		},
		{
			name:         "db-cpp path in zip",
			zipContents:  []string{"root/db-cpp/nested/file.txt"},
			expectedLang: "cpp",
		},
		{
			name:         "no db- path in zip",
			zipContents:  []string{"src/main.go", "log/build.log"},
			expectedLang: "unknown",
		},
		{
			name:         "empty zip",
			zipContents:  []string{},
			expectedLang: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			zipPath := filepath.Join(tempDir, tt.name+".zip")
			createTestZip(t, zipPath, tt.zipContents)

			lang := detectLanguageFromZip(zipPath)
			if lang != tt.expectedLang {
				t.Errorf("detectLanguageFromZip() = %q, want %q", lang, tt.expectedLang)
			}
		})
	}
}

func TestHashFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hash-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test file with known content
	testFile := filepath.Join(tempDir, "test.txt")
	content := []byte("hello world")
	if err := os.WriteFile(testFile, content, 0o644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Known SHA-256 hash for "hello world"
	expectedHash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	hash, err := hashFile(testFile)
	if err != nil {
		t.Fatalf("hashFile() error = %v", err)
	}

	if hash != expectedHash {
		t.Errorf("hashFile() = %q, want %q", hash, expectedHash)
	}
}

func TestHashFile_NonExistent(t *testing.T) {
	_, err := hashFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("hashFile() expected error for non-existent file, got nil")
	}
}

func TestDiscoverDatabases_EmptyDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "discover-empty-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	databases, err := DiscoverDatabases(tempDir)
	if err != nil {
		t.Fatalf("DiscoverDatabases() error = %v", err)
	}

	if len(databases) != 0 {
		t.Errorf("DiscoverDatabases() returned %d databases, want 0", len(databases))
	}
}

func TestDiscoverDatabases_UnarchivedDatabase(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "discover-unarchived-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock unarchived CodeQL database
	dbDir := filepath.Join(tempDir, "test-db")
	if err := os.MkdirAll(filepath.Join(dbDir, "db-go"), 0o755); err != nil {
		t.Fatalf("Failed to create db directory: %v", err)
	}

	// Create codeql-database.yml
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

	databases, err := DiscoverDatabases(tempDir)
	if err != nil {
		t.Fatalf("DiscoverDatabases() error = %v", err)
	}

	if len(databases) != 1 {
		t.Fatalf("DiscoverDatabases() returned %d databases, want 1", len(databases))
	}

	db := databases[0]
	if db.Language != "go" {
		t.Errorf("db.Language = %q, want %q", db.Language, "go")
	}
	if db.Owner != "owner" {
		t.Errorf("db.Owner = %q, want %q", db.Owner, "owner")
	}
	if db.Repo != "repo" {
		t.Errorf("db.Repo = %q, want %q", db.Repo, "repo")
	}
	if db.IsArchived {
		t.Errorf("db.IsArchived = true, want false")
	}
	if db.CreationMetadata == nil {
		t.Error("db.CreationMetadata is nil, want non-nil")
	} else {
		if db.CreationMetadata.SHA != "abc123" {
			t.Errorf("db.CreationMetadata.SHA = %q, want %q", db.CreationMetadata.SHA, "abc123")
		}
		if db.CreationMetadata.CLIVersion != "2.15.0" {
			t.Errorf("db.CreationMetadata.CLIVersion = %q, want %q", db.CreationMetadata.CLIVersion, "2.15.0")
		}
	}
}

func TestDiscoverDatabases_ArchivedDatabase(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "discover-archived-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock archived CodeQL database
	zipPath := filepath.Join(tempDir, "test-db.zip")
	yamlContent := `sourceLocationPrefix: /Users/test/src/owner/repo
primaryLanguage: javascript
creationMetadata:
  sha: def456
  cliVersion: 2.14.0
  creationTime: "2024-01-14T10:30:00Z"
`
	createTestZipWithYAML(t, zipPath, yamlContent, "db-javascript")

	databases, err := DiscoverDatabases(tempDir)
	if err != nil {
		t.Fatalf("DiscoverDatabases() error = %v", err)
	}

	if len(databases) != 1 {
		t.Fatalf("DiscoverDatabases() returned %d databases, want 1", len(databases))
	}

	db := databases[0]
	if db.Language != "javascript" {
		t.Errorf("db.Language = %q, want %q", db.Language, "javascript")
	}
	if db.Owner != "owner" {
		t.Errorf("db.Owner = %q, want %q", db.Owner, "owner")
	}
	if db.Repo != "repo" {
		t.Errorf("db.Repo = %q, want %q", db.Repo, "repo")
	}
	if !db.IsArchived {
		t.Errorf("db.IsArchived = false, want true")
	}
	if db.ContentHash == "" {
		t.Error("db.ContentHash is empty, want non-empty hash")
	}
}

func TestDiscoverDatabases_MixedDatabases(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "discover-mixed-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create an unarchived database
	dbDir := filepath.Join(tempDir, "unarchived-db")
	if err := os.MkdirAll(filepath.Join(dbDir, "db-go"), 0o755); err != nil {
		t.Fatalf("Failed to create db directory: %v", err)
	}
	yamlContent1 := `sourceLocationPrefix: /src/owner1/repo1
primaryLanguage: go
`
	if err := os.WriteFile(filepath.Join(dbDir, "codeql-database.yml"), []byte(yamlContent1), 0o644); err != nil {
		t.Fatalf("Failed to write codeql-database.yml: %v", err)
	}

	// Create an archived database
	zipPath := filepath.Join(tempDir, "archived-db.zip")
	yamlContent2 := `sourceLocationPrefix: /src/owner2/repo2
primaryLanguage: python
`
	createTestZipWithYAML(t, zipPath, yamlContent2, "db-python")

	databases, err := DiscoverDatabases(tempDir)
	if err != nil {
		t.Fatalf("DiscoverDatabases() error = %v", err)
	}

	if len(databases) != 2 {
		t.Fatalf("DiscoverDatabases() returned %d databases, want 2", len(databases))
	}

	// Check we got both types
	var hasArchived, hasUnarchived bool
	for _, db := range databases {
		if db.IsArchived {
			hasArchived = true
			if db.Language != "python" {
				t.Errorf("Archived db.Language = %q, want %q", db.Language, "python")
			}
		} else {
			hasUnarchived = true
			if db.Language != "go" {
				t.Errorf("Unarchived db.Language = %q, want %q", db.Language, "go")
			}
		}
	}

	if !hasArchived {
		t.Error("Missing archived database")
	}
	if !hasUnarchived {
		t.Error("Missing unarchived database")
	}
}

func TestDiscoverDatabases_NonExistentDirectory(t *testing.T) {
	_, err := DiscoverDatabases("/nonexistent/directory")
	if err == nil {
		t.Error("DiscoverDatabases() expected error for non-existent directory, got nil")
	}
}

func TestDiscoverDatabases_DBInfoFormat(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "discover-dbinfo-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a mock archived CodeQL database with old .dbinfo format
	zipPath := filepath.Join(tempDir, "u-boot_u-boot_cpp.zip")
	dbInfoContent := `<?xml version="1.0" encoding="UTF-8"?>
<dbinfo>
  <sourceLocationPrefix>/opt/src</sourceLocationPrefix>
</dbinfo>
`
	createTestZipWithDBInfo(t, zipPath, dbInfoContent, "db-cpp")

	databases, err := DiscoverDatabases(tempDir)
	if err != nil {
		t.Fatalf("DiscoverDatabases() error = %v", err)
	}

	if len(databases) != 1 {
		t.Fatalf("DiscoverDatabases() returned %d databases, want 1", len(databases))
	}

	db := databases[0]
	if db.Language != "cpp" {
		t.Errorf("db.Language = %q, want %q", db.Language, "cpp")
	}
	// Should extract from filename since sourceLocationPrefix is /opt/src
	if db.Owner != "u-boot" {
		t.Errorf("db.Owner = %q, want %q", db.Owner, "u-boot")
	}
	if db.Repo != "u-boot" {
		t.Errorf("db.Repo = %q, want %q", db.Repo, "u-boot")
	}
	if !db.IsArchived {
		t.Errorf("db.IsArchived = false, want true")
	}
	if db.CreationMetadata != nil {
		t.Errorf("db.CreationMetadata = %v, want nil for .dbinfo format", db.CreationMetadata)
	}
}

func TestDatabaseYAML_Parsing(t *testing.T) {
	// Test that DatabaseYAML struct correctly parses YAML
	yamlContent := `sourceLocationPrefix: /Users/test/src/owner/repo
primaryLanguage: java
unicodeNewlines: true
columnKind: utf16CodeUnits
creationMetadata:
  sha: abc123
  cliVersion: 2.15.0
  creationTime: "2024-01-15T10:30:00Z"
`
	var dbYAML DatabaseYAML
	if err := yaml.Unmarshal([]byte(yamlContent), &dbYAML); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}

	if dbYAML.SourceLocationPrefix != "/Users/test/src/owner/repo" {
		t.Errorf("SourceLocationPrefix = %q, want %q", dbYAML.SourceLocationPrefix, "/Users/test/src/owner/repo")
	}
	if dbYAML.PrimaryLanguage != "java" {
		t.Errorf("PrimaryLanguage = %q, want %q", dbYAML.PrimaryLanguage, "java")
	}
	if !dbYAML.UnicodeNewlines {
		t.Error("UnicodeNewlines = false, want true")
	}
	if dbYAML.ColumnKind != "utf16CodeUnits" {
		t.Errorf("ColumnKind = %q, want %q", dbYAML.ColumnKind, "utf16CodeUnits")
	}
	if dbYAML.CreationMetadata == nil {
		t.Fatal("CreationMetadata is nil")
	}
	if dbYAML.CreationMetadata.SHA != "abc123" {
		t.Errorf("CreationMetadata.SHA = %q, want %q", dbYAML.CreationMetadata.SHA, "abc123")
	}
}

func TestDBInfo_Parsing(t *testing.T) {
	// Test that DBInfo struct correctly parses XML
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<dbinfo>
  <sourceLocationPrefix>/opt/src/myproject</sourceLocationPrefix>
</dbinfo>
`
	var dbInfo DBInfo
	if err := xml.Unmarshal([]byte(xmlContent), &dbInfo); err != nil {
		t.Fatalf("xml.Unmarshal() error = %v", err)
	}

	if dbInfo.SourceLocationPrefix != "/opt/src/myproject" {
		t.Errorf("SourceLocationPrefix = %q, want %q", dbInfo.SourceLocationPrefix, "/opt/src/myproject")
	}
}

// Helper functions for creating test zip files

func createTestZip(t *testing.T, zipPath string, contents []string) {
	t.Helper()

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer file.Close()

	w := zip.NewWriter(file)
	defer w.Close()

	for _, path := range contents {
		_, err := w.Create(path)
		if err != nil {
			t.Fatalf("Failed to create zip entry: %v", err)
		}
	}
}

func createTestZipWithYAML(t *testing.T, zipPath, yamlContent, langDir string) {
	t.Helper()

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer file.Close()

	w := zip.NewWriter(file)

	// Add codeql-database.yml
	yamlWriter, err := w.Create("codeql-database.yml")
	if err != nil {
		t.Fatalf("Failed to create yaml entry: %v", err)
	}
	if _, err := yamlWriter.Write([]byte(yamlContent)); err != nil {
		t.Fatalf("Failed to write yaml content: %v", err)
	}

	// Add language directory
	if langDir != "" {
		_, err = w.Create(langDir + "/")
		if err != nil {
			t.Fatalf("Failed to create lang dir entry: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}
}

func createTestZipWithDBInfo(t *testing.T, zipPath, dbInfoContent, langDir string) {
	t.Helper()

	file, err := os.Create(zipPath)
	if err != nil {
		t.Fatalf("Failed to create zip file: %v", err)
	}
	defer file.Close()

	w := zip.NewWriter(file)

	// Add .dbinfo
	dbInfoWriter, err := w.Create(".dbinfo")
	if err != nil {
		t.Fatalf("Failed to create dbinfo entry: %v", err)
	}
	if _, err := dbInfoWriter.Write([]byte(dbInfoContent)); err != nil {
		t.Fatalf("Failed to write dbinfo content: %v", err)
	}

	// Add language directory
	if langDir != "" {
		_, err = w.Create(langDir + "/")
		if err != nil {
			t.Fatalf("Failed to create lang dir entry: %v", err)
		}
	}

	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close zip writer: %v", err)
	}
}
