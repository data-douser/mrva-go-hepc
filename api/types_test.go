// Package api tests for API types and JSON marshaling.
package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestDatabaseMetadata_JSONMarshal(t *testing.T) {
	meta := DatabaseMetadata{
		ContentHash:          "abc123",
		BuildCID:             "build-001",
		GitBranch:            "main",
		GitCommitID:          "commit123",
		GitOwner:             "testowner",
		GitRepo:              "testrepo",
		IngestionDatetimeUTC: "2024-01-15T10:30:00Z",
		PrimaryLanguage:      "go",
		ResultURL:            "http://localhost:8080/db/testrepo.zip",
		ToolName:             "codeql-go",
		ToolVersion:          "2.15.0",
		Projname:             "testowner/testrepo",
		DBFileSize:           1024000,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Verify all JSON field names are correct
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	expectedFields := map[string]interface{}{
		"content_hash":           "abc123",
		"build_cid":              "build-001",
		"git_branch":             "main",
		"git_commit_id":          "commit123",
		"git_owner":              "testowner",
		"git_repo":               "testrepo",
		"ingestion_datetime_utc": "2024-01-15T10:30:00Z",
		"primary_language":       "go",
		"result_url":             "http://localhost:8080/db/testrepo.zip",
		"tool_name":              "codeql-go",
		"tool_version":           "2.15.0",
		"projname":               "testowner/testrepo",
		"db_file_size":           float64(1024000), // JSON numbers are float64
	}

	for key, expected := range expectedFields {
		actual, ok := result[key]
		if !ok {
			t.Errorf("missing JSON field %q", key)
			continue
		}
		if actual != expected {
			t.Errorf("field %q = %v, want %v", key, actual, expected)
		}
	}
}

func TestDatabaseMetadata_JSONUnmarshal(t *testing.T) {
	jsonData := `{
		"content_hash": "def456",
		"build_cid": "build-002",
		"git_branch": "develop",
		"git_commit_id": "commit456",
		"git_owner": "anotherowner",
		"git_repo": "anotherrepo",
		"ingestion_datetime_utc": "2024-02-20T15:45:00Z",
		"primary_language": "python",
		"result_url": "http://example.com/db/repo.zip",
		"tool_name": "codeql-python",
		"tool_version": "2.16.0",
		"projname": "anotherowner/anotherrepo",
		"db_file_size": 2048000
	}`

	var meta DatabaseMetadata
	if err := json.Unmarshal([]byte(jsonData), &meta); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if meta.ContentHash != "def456" {
		t.Errorf("ContentHash = %q, want %q", meta.ContentHash, "def456")
	}
	if meta.BuildCID != "build-002" {
		t.Errorf("BuildCID = %q, want %q", meta.BuildCID, "build-002")
	}
	if meta.GitBranch != "develop" {
		t.Errorf("GitBranch = %q, want %q", meta.GitBranch, "develop")
	}
	if meta.GitCommitID != "commit456" {
		t.Errorf("GitCommitID = %q, want %q", meta.GitCommitID, "commit456")
	}
	if meta.GitOwner != "anotherowner" {
		t.Errorf("GitOwner = %q, want %q", meta.GitOwner, "anotherowner")
	}
	if meta.GitRepo != "anotherrepo" {
		t.Errorf("GitRepo = %q, want %q", meta.GitRepo, "anotherrepo")
	}
	if meta.IngestionDatetimeUTC != "2024-02-20T15:45:00Z" {
		t.Errorf("IngestionDatetimeUTC = %q, want %q", meta.IngestionDatetimeUTC, "2024-02-20T15:45:00Z")
	}
	if meta.PrimaryLanguage != "python" {
		t.Errorf("PrimaryLanguage = %q, want %q", meta.PrimaryLanguage, "python")
	}
	if meta.ResultURL != "http://example.com/db/repo.zip" {
		t.Errorf("ResultURL = %q, want %q", meta.ResultURL, "http://example.com/db/repo.zip")
	}
	if meta.ToolName != "codeql-python" {
		t.Errorf("ToolName = %q, want %q", meta.ToolName, "codeql-python")
	}
	if meta.ToolVersion != "2.16.0" {
		t.Errorf("ToolVersion = %q, want %q", meta.ToolVersion, "2.16.0")
	}
	if meta.Projname != "anotherowner/anotherrepo" {
		t.Errorf("Projname = %q, want %q", meta.Projname, "anotherowner/anotherrepo")
	}
	if meta.DBFileSize != 2048000 {
		t.Errorf("DBFileSize = %d, want %d", meta.DBFileSize, 2048000)
	}
}

func TestDatabaseMetadata_RoundTrip(t *testing.T) {
	original := DatabaseMetadata{
		ContentHash:          "roundtrip123",
		BuildCID:             "build-rt",
		GitBranch:            "feature/test",
		GitCommitID:          "abc123def456",
		GitOwner:             "myorg",
		GitRepo:              "myproject",
		IngestionDatetimeUTC: "2024-03-01T00:00:00Z",
		PrimaryLanguage:      "javascript",
		ResultURL:            "https://storage.example.com/db.zip",
		ToolName:             "codeql-javascript",
		ToolVersion:          "2.14.0",
		Projname:             "myorg/myproject",
		DBFileSize:           5000000,
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal back
	var restored DatabaseMetadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Compare
	if !reflect.DeepEqual(original, restored) {
		t.Errorf("round-trip mismatch:\noriginal: %+v\nrestored: %+v", original, restored)
	}
}

func TestDatabaseMetadata_ZeroValue(t *testing.T) {
	var meta DatabaseMetadata

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Zero values should marshal without error
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// All string fields should be empty strings
	stringFields := []string{
		"content_hash", "build_cid", "git_branch", "git_commit_id",
		"git_owner", "git_repo", "ingestion_datetime_utc", "primary_language",
		"result_url", "tool_name", "tool_version", "projname",
	}

	for _, field := range stringFields {
		val, ok := result[field]
		if !ok {
			t.Errorf("missing field %q in zero-value marshal", field)
			continue
		}
		if val != "" {
			t.Errorf("field %q = %v, want empty string", field, val)
		}
	}

	// DBFileSize should be 0
	if result["db_file_size"] != float64(0) {
		t.Errorf("db_file_size = %v, want 0", result["db_file_size"])
	}
}

func TestDatabaseMetadata_PartialJSON(t *testing.T) {
	// Test that partial JSON (missing fields) unmarshals correctly
	jsonData := `{
		"content_hash": "partial123",
		"git_owner": "owner",
		"git_repo": "repo",
		"primary_language": "java"
	}`

	var meta DatabaseMetadata
	if err := json.Unmarshal([]byte(jsonData), &meta); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if meta.ContentHash != "partial123" {
		t.Errorf("ContentHash = %q, want %q", meta.ContentHash, "partial123")
	}
	if meta.GitOwner != "owner" {
		t.Errorf("GitOwner = %q, want %q", meta.GitOwner, "owner")
	}
	if meta.GitRepo != "repo" {
		t.Errorf("GitRepo = %q, want %q", meta.GitRepo, "repo")
	}
	if meta.PrimaryLanguage != "java" {
		t.Errorf("PrimaryLanguage = %q, want %q", meta.PrimaryLanguage, "java")
	}

	// Missing fields should be zero values
	if meta.BuildCID != "" {
		t.Errorf("BuildCID = %q, want empty", meta.BuildCID)
	}
	if meta.DBFileSize != 0 {
		t.Errorf("DBFileSize = %d, want 0", meta.DBFileSize)
	}
}

func TestMetadataResponse_JSONMarshal(t *testing.T) {
	response := MetadataResponse{
		{
			ContentHash:     "hash1",
			GitOwner:        "owner1",
			GitRepo:         "repo1",
			PrimaryLanguage: "go",
		},
		{
			ContentHash:     "hash2",
			GitOwner:        "owner2",
			GitRepo:         "repo2",
			PrimaryLanguage: "python",
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Should be a JSON array
	var result []map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if len(result) != 2 {
		t.Errorf("response length = %d, want 2", len(result))
	}

	if result[0]["content_hash"] != "hash1" {
		t.Errorf("first entry content_hash = %v, want %q", result[0]["content_hash"], "hash1")
	}
	if result[1]["content_hash"] != "hash2" {
		t.Errorf("second entry content_hash = %v, want %q", result[1]["content_hash"], "hash2")
	}
}

func TestMetadataResponse_EmptySlice(t *testing.T) {
	response := MetadataResponse{}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Empty slice should marshal to "[]"
	if string(data) != "[]" {
		t.Errorf("empty MetadataResponse marshal = %s, want []", string(data))
	}
}

func TestMetadataResponse_NilSlice(t *testing.T) {
	var response MetadataResponse = nil

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Nil slice should marshal to "null"
	if string(data) != "null" {
		t.Errorf("nil MetadataResponse marshal = %s, want null", string(data))
	}
}

func TestDatabaseMetadata_DBTags(t *testing.T) {
	// Verify that the struct has correct db tags for database operations
	typ := reflect.TypeOf(DatabaseMetadata{})

	expectedDBTags := map[string]string{
		"ContentHash":          "content_hash",
		"BuildCID":             "build_cid",
		"GitBranch":            "git_branch",
		"GitCommitID":          "git_commit_id",
		"GitOwner":             "git_owner",
		"GitRepo":              "git_repo",
		"IngestionDatetimeUTC": "ingestion_datetime_utc",
		"PrimaryLanguage":      "primary_language",
		"ResultURL":            "result_url",
		"ToolName":             "tool_name",
		"ToolVersion":          "tool_version",
		"Projname":             "projname",
		"DBFileSize":           "db_file_size",
	}

	for fieldName, expectedTag := range expectedDBTags {
		field, ok := typ.FieldByName(fieldName)
		if !ok {
			t.Errorf("field %q not found", fieldName)
			continue
		}

		dbTag := field.Tag.Get("db")
		if dbTag != expectedTag {
			t.Errorf("field %q db tag = %q, want %q", fieldName, dbTag, expectedTag)
		}
	}
}

func TestDatabaseMetadata_JSONTags(t *testing.T) {
	// Verify that the struct has correct json tags
	typ := reflect.TypeOf(DatabaseMetadata{})

	expectedJSONTags := map[string]string{
		"ContentHash":          "content_hash",
		"BuildCID":             "build_cid",
		"GitBranch":            "git_branch",
		"GitCommitID":          "git_commit_id",
		"GitOwner":             "git_owner",
		"GitRepo":              "git_repo",
		"IngestionDatetimeUTC": "ingestion_datetime_utc",
		"PrimaryLanguage":      "primary_language",
		"ResultURL":            "result_url",
		"ToolName":             "tool_name",
		"ToolVersion":          "tool_version",
		"Projname":             "projname",
		"DBFileSize":           "db_file_size",
	}

	for fieldName, expectedTag := range expectedJSONTags {
		field, ok := typ.FieldByName(fieldName)
		if !ok {
			t.Errorf("field %q not found", fieldName)
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag != expectedTag {
			t.Errorf("field %q json tag = %q, want %q", fieldName, jsonTag, expectedTag)
		}
	}
}

func TestDatabaseMetadata_SpecialCharacters(t *testing.T) {
	// Test handling of special characters in fields
	meta := DatabaseMetadata{
		ContentHash:     "hash<>\"'&",
		GitOwner:        "owner-with-dashes",
		GitRepo:         "repo_with_underscores",
		GitBranch:       "feature/branch-name",
		PrimaryLanguage: "c++", // Note: invalid language but tests encoding
		Projname:        "owner/repo",
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var restored DatabaseMetadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if restored.ContentHash != meta.ContentHash {
		t.Errorf("ContentHash = %q, want %q", restored.ContentHash, meta.ContentHash)
	}
	if restored.GitBranch != meta.GitBranch {
		t.Errorf("GitBranch = %q, want %q", restored.GitBranch, meta.GitBranch)
	}
	if restored.PrimaryLanguage != meta.PrimaryLanguage {
		t.Errorf("PrimaryLanguage = %q, want %q", restored.PrimaryLanguage, meta.PrimaryLanguage)
	}
}

func TestDatabaseMetadata_LargeFileSize(t *testing.T) {
	// Test handling of large file sizes (multi-GB databases)
	meta := DatabaseMetadata{
		ContentHash: "large",
		DBFileSize:  10 * 1024 * 1024 * 1024, // 10 GB
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var restored DatabaseMetadata
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if restored.DBFileSize != meta.DBFileSize {
		t.Errorf("DBFileSize = %d, want %d", restored.DBFileSize, meta.DBFileSize)
	}
}
