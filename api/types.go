// Package api defines the data types for the MRVA HEPC interface.
package api

// DatabaseMetadata represents metadata for a CodeQL database entry.
// This matches the schema from the Python reference implementation's SQLite table.
type DatabaseMetadata struct {
	// ContentHash is the SHA-256 hash of the database file (primary key in SQLite).
	ContentHash string `json:"content_hash" db:"content_hash"`

	// BuildCID is the context/build identifier generated from CLI version,
	// creation time, primary language, and source SHA.
	BuildCID string `json:"build_cid" db:"build_cid"`

	// GitBranch is the git branch name (typically "HEAD").
	GitBranch string `json:"git_branch" db:"git_branch"`

	// GitCommitID is the git commit SHA of the analyzed source code.
	GitCommitID string `json:"git_commit_id" db:"git_commit_id"`

	// GitOwner is the repository owner (e.g., GitHub org or username).
	GitOwner string `json:"git_owner" db:"git_owner"`

	// GitRepo is the repository name.
	GitRepo string `json:"git_repo" db:"git_repo"`

	// IngestionDatetimeUTC is the creation timestamp of the CodeQL database.
	IngestionDatetimeUTC string `json:"ingestion_datetime_utc" db:"ingestion_datetime_utc"`

	// PrimaryLanguage is the primary programming language of the database
	// (e.g., "javascript", "python", "go").
	PrimaryLanguage string `json:"primary_language" db:"primary_language"`

	// ResultURL is the URL where the database file can be downloaded.
	ResultURL string `json:"result_url" db:"result_url"`

	// ToolName is the name of the CodeQL tool (e.g., "codeql-javascript").
	ToolName string `json:"tool_name" db:"tool_name"`

	// ToolVersion is the version of the CodeQL CLI used to create the database.
	ToolVersion string `json:"tool_version" db:"tool_version"`

	// Projname is the project name in "owner/repo" format.
	Projname string `json:"projname" db:"projname"`

	// DBFileSize is the size of the database file in bytes.
	DBFileSize int64 `json:"db_file_size" db:"db_file_size"`
}

// MetadataResponse is returned by index and API endpoints.
// Each line in the response is a JSON-encoded DatabaseMetadata record (JSONL format).
type MetadataResponse []DatabaseMetadata
