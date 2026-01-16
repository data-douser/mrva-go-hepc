// Package codeql provides utilities for discovering CodeQL databases
// and extracting their metadata.
package codeql

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DatabaseYAML represents the structure of codeql-database.yml file.
// This is the modern CodeQL database metadata format.
type DatabaseYAML struct {
	SourceLocationPrefix string                    `yaml:"sourceLocationPrefix"`
	PrimaryLanguage      string                    `yaml:"primaryLanguage"`
	UnicodeNewlines      bool                      `yaml:"unicodeNewlines"`
	ColumnKind           string                    `yaml:"columnKind"`
	CreationMetadata     *DatabaseCreationMetadata `yaml:"creationMetadata,omitempty"`
}

// DatabaseCreationMetadata holds creation details from codeql-database.yml.
type DatabaseCreationMetadata struct {
	SHA          string `yaml:"sha"`
	CLIVersion   string `yaml:"cliVersion"`
	CreationTime string `yaml:"creationTime"`
}

// DBInfo represents the older XML .dbinfo format.
type DBInfo struct {
	XMLName              xml.Name `xml:"dbinfo"`
	SourceLocationPrefix string   `xml:"sourceLocationPrefix"`
}

// DiscoveredDatabase represents a discovered CodeQL database with its metadata.
type DiscoveredDatabase struct {
	// Path is the full path to the database (directory or archive file).
	Path string

	// Name is the base name of the database.
	Name string

	// IsArchived indicates if the database is a zip archive.
	IsArchived bool

	// Language is the primary programming language.
	Language string

	// SourceLocationPrefix is the original source path.
	SourceLocationPrefix string

	// CreationMetadata from codeql-database.yml (may be nil for older formats).
	CreationMetadata *DatabaseCreationMetadata

	// FileSize is the size of the database archive or directory.
	FileSize int64

	// ContentHash is the SHA-256 hash of the database (for archives only).
	ContentHash string

	// Owner and Repo extracted from source location prefix.
	Owner string
	Repo  string
}

// DiscoverDatabases recursively scans a directory for CodeQL databases.
// It finds both archived (.zip) and unarchived databases.
func DiscoverDatabases(basePath string) ([]*DiscoveredDatabase, error) {
	var databases []*DiscoveredDatabase

	err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the base path itself
		if path == basePath {
			return nil
		}

		// Check for zip archives
		if !d.IsDir() && strings.HasSuffix(strings.ToLower(d.Name()), ".zip") {
			db, err := discoverArchivedDatabase(path)
			if err != nil {
				// Log but continue - not all zips are CodeQL databases
				fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", path, err)
				return nil
			}
			if db != nil {
				databases = append(databases, db)
			}
			return nil
		}

		// Check for unarchived databases (directories with codeql-database.yml)
		if d.IsDir() {
			ymlPath := filepath.Join(path, "codeql-database.yml")
			if _, err := os.Stat(ymlPath); err == nil {
				db, err := discoverUnarchivedDatabase(path)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", path, err)
					return nil
				}
				if db != nil {
					databases = append(databases, db)
					// Skip descending into this directory
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	return databases, err
}

// discoverArchivedDatabase extracts metadata from a zip-archived CodeQL database.
func discoverArchivedDatabase(zipPath string) (*DiscoveredDatabase, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open zip: %w", err)
	}
	defer func() {
		_ = reader.Close() //nolint:errcheck // Best effort close in defer
	}()

	// Try to find codeql-database.yml first
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, "codeql-database.yml") {
			db, err := extractMetadataFromYAMLFile(f, zipPath, true)
			if err != nil {
				return nil, err
			}
			return db, nil
		}
	}

	// Fall back to .dbinfo (older format)
	for _, f := range reader.File {
		if strings.HasSuffix(f.Name, ".dbinfo") {
			db, err := extractMetadataFromDBInfo(f, zipPath)
			if err != nil {
				return nil, err
			}
			return db, nil
		}
	}

	return nil, nil // Not a CodeQL database
}

// discoverUnarchivedDatabase extracts metadata from an unarchived CodeQL database.
func discoverUnarchivedDatabase(dbPath string) (*DiscoveredDatabase, error) {
	ymlPath := filepath.Join(dbPath, "codeql-database.yml")

	data, err := os.ReadFile(ymlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read codeql-database.yml: %w", err)
	}

	var dbYAML DatabaseYAML
	if unmarshalErr := yaml.Unmarshal(data, &dbYAML); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse codeql-database.yml: %w", unmarshalErr)
	}

	// Calculate directory size
	var totalSize int64
	err = filepath.WalkDir(dbPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				totalSize += info.Size()
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to calculate directory size: %w", err)
	}

	// Determine language - use primaryLanguage from YAML, or detect from db-<lang> directories
	language := dbYAML.PrimaryLanguage
	if language == "" {
		language = detectLanguageFromDirectory(dbPath)
	}

	// Extract owner/repo from source location prefix
	owner, repo := extractOwnerRepo(dbYAML.SourceLocationPrefix)

	db := &DiscoveredDatabase{
		Path:                 dbPath,
		Name:                 filepath.Base(dbPath),
		IsArchived:           false,
		Language:             language,
		SourceLocationPrefix: dbYAML.SourceLocationPrefix,
		CreationMetadata:     dbYAML.CreationMetadata,
		FileSize:             totalSize,
		Owner:                owner,
		Repo:                 repo,
	}

	return db, nil
}

// extractMetadataFromYAMLFile extracts metadata from a codeql-database.yml inside a zip.
func extractMetadataFromYAMLFile(f *zip.File, zipPath string, isArchived bool) (*DiscoveredDatabase, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open yaml file in zip: %w", err)
	}
	defer func() {
		_ = rc.Close() //nolint:errcheck // Best effort close in defer
	}()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read yaml file: %w", err)
	}

	var dbYAML DatabaseYAML
	if unmarshalErr := yaml.Unmarshal(data, &dbYAML); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse codeql-database.yml: %w", unmarshalErr)
	}

	// Get zip file info
	info, err := os.Stat(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat zip file: %w", err)
	}

	// Calculate content hash
	contentHash, err := hashFile(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to hash file: %w", err)
	}

	// Extract owner/repo from source location prefix
	owner, repo := extractOwnerRepo(dbYAML.SourceLocationPrefix)

	// Determine language - use primaryLanguage from YAML, or detect from db-<lang> directories in zip
	language := dbYAML.PrimaryLanguage
	if language == "" {
		language = detectLanguageFromZip(zipPath)
	}

	db := &DiscoveredDatabase{
		Path:                 zipPath,
		Name:                 filepath.Base(zipPath),
		IsArchived:           isArchived,
		Language:             language,
		SourceLocationPrefix: dbYAML.SourceLocationPrefix,
		CreationMetadata:     dbYAML.CreationMetadata,
		FileSize:             info.Size(),
		ContentHash:          contentHash,
		Owner:                owner,
		Repo:                 repo,
	}

	return db, nil
}

// extractMetadataFromDBInfo extracts metadata from a .dbinfo XML file inside a zip.
func extractMetadataFromDBInfo(f *zip.File, zipPath string) (*DiscoveredDatabase, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open dbinfo file in zip: %w", err)
	}
	defer func() {
		_ = rc.Close() //nolint:errcheck // Best effort close in defer
	}()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read dbinfo file: %w", err)
	}

	var dbInfo DBInfo
	if unmarshalErr := xml.Unmarshal(data, &dbInfo); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to parse .dbinfo: %w", unmarshalErr)
	}

	// Get zip file info
	info, err := os.Stat(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat zip file: %w", err)
	}

	// Calculate content hash
	contentHash, err := hashFile(zipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to hash file: %w", err)
	}

	// Try to detect language from the zip contents
	language := detectLanguageFromZip(zipPath)

	// Extract owner/repo from source location prefix
	owner, repo := extractOwnerRepo(dbInfo.SourceLocationPrefix)

	// For old-format databases, try to extract owner/repo from the filename
	// if the source location prefix doesn't give useful info
	if owner == "opt" || owner == "src" || owner == "unknown" {
		filenameOwner, filenameRepo := extractOwnerRepoFromFilename(filepath.Base(zipPath))
		if filenameOwner != "unknown" {
			owner = filenameOwner
		}
		if filenameRepo != "unknown" {
			repo = filenameRepo
		}
	}

	db := &DiscoveredDatabase{
		Path:                 zipPath,
		Name:                 filepath.Base(zipPath),
		IsArchived:           true,
		Language:             language,
		SourceLocationPrefix: dbInfo.SourceLocationPrefix,
		CreationMetadata:     nil, // Old format doesn't have this
		FileSize:             info.Size(),
		ContentHash:          contentHash,
		Owner:                owner,
		Repo:                 repo,
	}

	return db, nil
}

// extractOwnerRepo extracts owner and repo from a source location prefix path.
// For paths like "/Users/xavier/src/github/titus-control-plane", it extracts
// "github" as owner and "titus-control-plane" as repo.
func extractOwnerRepo(sourceLocationPrefix string) (owner, repo string) {
	if sourceLocationPrefix == "" {
		return "unknown", "unknown"
	}

	// Clean and split the path
	parts := strings.Split(filepath.Clean(sourceLocationPrefix), string(filepath.Separator))

	// Filter out empty parts
	var filteredParts []string
	for _, p := range parts {
		if p != "" {
			filteredParts = append(filteredParts, p)
		}
	}

	// Get the last two parts as owner and repo
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

// extractOwnerRepoFromFilename attempts to extract owner/repo from a database filename.
// For filenames like "owner_repo_lang-srcVersion_hash-dist_date.zip", it extracts owner and repo.
// This is a best-effort extraction for older database formats.
func extractOwnerRepoFromFilename(filename string) (owner, repo string) {
	// Remove common extensions
	name := strings.TrimSuffix(filename, ".zip")
	name = strings.TrimSuffix(name, ".tar.gz")
	name = strings.TrimSuffix(name, ".tgz")

	// Try to split on underscores (common pattern: owner_repo_...)
	parts := strings.Split(name, "_")
	if len(parts) >= 2 {
		owner = parts[0]
		repo = parts[1]
		return owner, repo
	}

	// Try to split on hyphens (another common pattern)
	parts = strings.Split(name, "-")
	if len(parts) >= 2 {
		owner = parts[0]
		repo = parts[1]
		return owner, repo
	}

	return "unknown", "unknown"
}

// hashFile computes the SHA-256 hash of a file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = f.Close() //nolint:errcheck // Best effort close in defer
	}()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// detectLanguageFromZip tries to detect the primary language from a zip file
// by looking for db-<language> directories or paths containing db-<language>/.
func detectLanguageFromZip(zipPath string) string {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "unknown"
	}
	defer func() {
		_ = reader.Close() //nolint:errcheck // Best effort close in defer
	}()

	// Look for paths containing db-<language>/
	for _, f := range reader.File {
		// Check if path contains db-<language>/ pattern
		parts := strings.Split(f.Name, "/")
		for _, part := range parts {
			if strings.HasPrefix(part, "db-") {
				lang := strings.TrimPrefix(part, "db-")
				if lang != "" {
					return lang
				}
			}
		}
	}

	return "unknown"
}

// detectLanguageFromDirectory tries to detect the primary language from an unarchived
// database directory by looking for db-<language> subdirectories.
func detectLanguageFromDirectory(dbPath string) string {
	entries, err := os.ReadDir(dbPath)
	if err != nil {
		return "unknown"
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "db-") {
			lang := strings.TrimPrefix(entry.Name(), "db-")
			if lang != "" {
				return lang
			}
		}
	}

	return "unknown"
}
