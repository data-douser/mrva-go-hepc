// hepc-server is the main entry point for the MRVA HEPC HTTP server.
//
// It serves CodeQL database files and metadata from a specified storage backend,
// implementing the MRVA HEPC interface for database discovery and download.
//
// Supported storage backends:
//   - local: Local filesystem storage
//   - gcs: Google Cloud Storage
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/data-douser/mrva-go-hepc/internal/server"
	"github.com/data-douser/mrva-go-hepc/internal/storage"
	"github.com/data-douser/mrva-go-hepc/internal/storage/gcs"
	"github.com/data-douser/mrva-go-hepc/internal/storage/local"
)

const (
	defaultHost        = "127.0.0.1"
	defaultPort        = 8070
	defaultStorageType = "local"
)

// storageConfig holds parsed storage configuration.
type storageConfig struct {
	storageType    string
	dbDir          string
	endpointURL    string
	gcsBucket      string
	gcsPrefix      string
	gcsCredentials string
	gcsCacheDir    string
	host           string
	port           int
}

// initStorage creates and initializes the appropriate storage backend.
func initStorage(ctx context.Context, cfg storageConfig, logger *slog.Logger) (storage.Backend, error) {
	epURL := cfg.endpointURL
	if epURL == "" {
		epURL = fmt.Sprintf("http://%s:%d", cfg.host, cfg.port)
	}

	switch cfg.storageType {
	case "local":
		if cfg.dbDir == "" {
			return nil, fmt.Errorf("--db-dir is required for local storage")
		}
		store, err := local.New(local.Config{
			BasePath:    cfg.dbDir,
			EndpointURL: epURL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize local storage: %w", err)
		}
		logger.Info("initialized local storage", "path", cfg.dbDir, "endpoint", epURL)
		return store, nil

	case "gcs":
		if cfg.gcsBucket == "" {
			return nil, fmt.Errorf("--gcs-bucket is required for gcs storage")
		}
		store, err := gcs.New(ctx, gcs.Config{
			Bucket:          cfg.gcsBucket,
			Prefix:          cfg.gcsPrefix,
			CredentialsFile: cfg.gcsCredentials,
			LocalCacheDir:   cfg.gcsCacheDir,
			EndpointURL:     epURL,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to initialize GCS storage: %w", err)
		}
		logger.Info("initialized GCS storage",
			"bucket", cfg.gcsBucket,
			"prefix", cfg.gcsPrefix,
			"endpoint", epURL,
		)
		return store, nil

	default:
		return nil, fmt.Errorf("unknown storage type: %s (supported: local, gcs)", cfg.storageType)
	}
}

func main() {
	// Define command-line flags
	// Server flags
	host := flag.String("host", defaultHost, "Host address for the HTTP server")
	port := flag.Int("port", defaultPort, "Port for the HTTP server")

	// Storage selection
	storageType := flag.String("storage", defaultStorageType, "Storage backend type: 'local' or 'gcs'")

	// Local storage flags
	dbDir := flag.String("db-dir", "", "Directory containing CodeQL database files (required for local storage)")
	endpointURL := flag.String("endpoint-url", "", "Base URL for result URLs (e.g., http://localhost:8070)")

	// GCS storage flags
	gcsBucket := flag.String("gcs-bucket", "", "GCS bucket name (required for gcs storage)")
	gcsPrefix := flag.String("gcs-prefix", "", "Object path prefix within the GCS bucket")
	gcsCredentials := flag.String("gcs-credentials", "", "Path to GCS service account JSON key file (uses ADC if not specified)")
	gcsCacheDir := flag.String("gcs-cache-dir", "", "Local directory for caching GCS metadata (uses temp dir if not specified)")

	help := flag.Bool("help", false, "Show help message")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `hepc-server - MRVA HTTP Endpoint for CodeQL databases

USAGE:
    hepc-server --storage <type> [STORAGE OPTIONS] [SERVER OPTIONS]

DESCRIPTION:
    Serves CodeQL database .zip files and metadata from a storage backend.
    
    The HTTP endpoints are:
      GET /db/{filename}                    - Download a database file
      GET /index                            - List all databases (JSONL)
      GET /api/v1/latest_results/codeql-all - List all databases (JSONL)
      GET /health                           - Health check endpoint

STORAGE BACKENDS:
    local   Local filesystem storage (default)
    gcs     Google Cloud Storage

SERVER OPTIONS:
`)
		flag.VisitAll(func(f *flag.Flag) {
			if f.Name == "host" || f.Name == "port" || f.Name == "help" {
				fmt.Fprintf(os.Stderr, "    --%s\n        %s (default: %v)\n", f.Name, f.Usage, f.DefValue)
			}
		})

		fmt.Fprintf(os.Stderr, `
LOCAL STORAGE OPTIONS:
    --db-dir <directory>
        Directory containing CodeQL database files (required)

GCS STORAGE OPTIONS:
    --gcs-bucket <bucket>
        GCS bucket name (required)
    --gcs-prefix <prefix>
        Object path prefix within the bucket (optional)
    --gcs-credentials <path>
        Path to service account JSON key file
        If not specified, uses Application Default Credentials (ADC)
    --gcs-cache-dir <directory>
        Local directory for caching metadata (optional)

EXAMPLES:
    # Local filesystem storage
    hepc-server --storage local --db-dir ./db-collection

    # Google Cloud Storage with ADC
    hepc-server --storage gcs --gcs-bucket my-codeql-dbs

    # GCS with service account and prefix
    hepc-server --storage gcs \
        --gcs-bucket my-codeql-dbs \
        --gcs-prefix databases/production/ \
        --gcs-credentials /path/to/service-account.json

AUTHENTICATION (GCS):
    The GCS backend supports multiple authentication methods:
    
    1. Service Account Key File:
       Use --gcs-credentials to specify a JSON key file
    
    2. Application Default Credentials (ADC):
       If no credentials file is specified, the client uses ADC:
       - GOOGLE_APPLICATION_CREDENTIALS environment variable
       - gcloud auth application-default login
       - Compute Engine/GKE service account (when running on GCP)

`)
	}

	flag.Parse()

	if *help {
		flag.Usage()
		os.Exit(0)
	}

	// Set up structured logging
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create storage backend
	ctx := context.Background()
	store, err := initStorage(ctx, storageConfig{
		storageType:    *storageType,
		dbDir:          *dbDir,
		endpointURL:    *endpointURL,
		gcsBucket:      *gcsBucket,
		gcsPrefix:      *gcsPrefix,
		gcsCredentials: *gcsCredentials,
		gcsCacheDir:    *gcsCacheDir,
		host:           *host,
		port:           *port,
	}, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		flag.Usage()
		os.Exit(1)
	}
	defer func() {
		if closeErr := store.Close(); closeErr != nil {
			logger.Error("failed to close storage", "error", closeErr)
		}
	}()

	// Create server configuration
	cfg := server.Config{
		Host: *host,
		Port: *port,
	}

	// Create and start the server
	srv := server.New(cfg, store, logger)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("received signal, initiating shutdown", "signal", sig)
		cancel()
	}()

	// Run the server
	if err := srv.ListenAndServe(ctx); err != nil {
		// http.ErrServerClosed is expected during graceful shutdown
		if err.Error() != "http: Server closed" {
			logger.Error("server error", "error", err)
			cancel()
			logger.Info("server stopped")
			return
		}
	}

	logger.Info("server stopped")
}
