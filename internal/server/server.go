// Package server provides the HTTP server implementation for MRVA HEPC.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/data-douser/mrva-go-hepc/internal/storage"
)

// Config holds the server configuration.
type Config struct {
	// Host is the address to bind the HTTP server to.
	Host string
	// Port is the port number for the HTTP server.
	Port int
}

// Server represents the HEPC HTTP server.
type Server struct {
	config  Config
	logger  *slog.Logger
	mux     *http.ServeMux
	storage storage.Backend
}

// New creates a new Server instance with the given configuration and storage backend.
func New(cfg Config, store storage.Backend, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}

	s := &Server{
		config:  cfg,
		logger:  logger,
		mux:     http.NewServeMux(),
		storage: store,
	}

	s.registerRoutes()
	return s
}

// registerRoutes sets up the HTTP route handlers.
func (s *Server) registerRoutes() {
	// Database file serving endpoint
	s.mux.HandleFunc("GET /db/{filepath...}", s.handleServeFile)

	// Metadata endpoints (both return the same data)
	s.mux.HandleFunc("GET /index", s.handleMetadata)
	s.mux.HandleFunc("GET /api/v1/latest_results/codeql-all", s.handleMetadata)

	// Health check endpoint
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

// Handler returns the HTTP handler for the server.
func (s *Server) Handler() http.Handler {
	return s.loggingMiddleware(s.mux)
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	srv := &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 30 * time.Second,
	}

	// Handle graceful shutdown
	go func() {
		<-ctx.Done()
		s.logger.Info("shutting down server")
		if err := srv.Shutdown(context.Background()); err != nil {
			s.logger.Error("shutdown error", "error", err)
		}
	}()

	s.logger.Info("starting server",
		"addr", addr,
		"storage_type", s.storage.Type(),
	)
	return srv.ListenAndServe()
}

// loggingMiddleware logs all incoming HTTP requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Info("incoming request", "method", r.Method, "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// handleServeFile serves database files from the storage backend.
func (s *Server) handleServeFile(w http.ResponseWriter, r *http.Request) {
	// Extract the filepath from the URL pattern
	requestedPath := r.PathValue("filepath")
	if requestedPath == "" {
		http.Error(w, "file path required", http.StatusBadRequest)
		return
	}

	s.logger.Info("serving file", "requested", requestedPath)

	reader, size, contentType, err := s.storage.GetFile(r.Context(), requestedPath)
	if err != nil {
		var notFound *storage.ErrNotFound
		if errors.As(err, &notFound) {
			s.logger.Error("file not found", "path", requestedPath)
			http.Error(w, fmt.Sprintf("%s not found", requestedPath), http.StatusNotFound)
			return
		}
		s.logger.Error("error accessing file", "path", requestedPath, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	defer func() {
		if err := reader.Close(); err != nil {
			s.logger.Error("failed to close reader", "error", err)
		}
	}()

	// Set response headers
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))

	// Stream the file content
	if _, err := io.Copy(w, reader); err != nil {
		s.logger.Error("error streaming file", "path", requestedPath, "error", err)
		// Can't set error status here as headers are already sent
	}
}

// handleMetadata serves metadata from the storage backend as JSONL.
func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	s.logger.Info("serving metadata")

	exists, err := s.storage.MetadataExists(r.Context())
	if err != nil {
		s.logger.Error("error checking metadata existence", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !exists {
		s.logger.Error("metadata database not found")
		http.Error(w, "metadata.sql not found", http.StatusNotFound)
		return
	}

	metadata, err := s.storage.ListMetadata(r.Context())
	if err != nil {
		s.logger.Error("error loading metadata", "error", err)
		http.Error(w, fmt.Sprintf("database error: %v", err), http.StatusInternalServerError)
		return
	}

	// Write as JSONL (newline-delimited JSON)
	w.Header().Set("Content-Type", "application/x-ndjson")

	var lines []string
	for i := range metadata {
		line, err := json.Marshal(metadata[i])
		if err != nil {
			s.logger.Error("error marshaling metadata", "error", err)
			continue
		}
		lines = append(lines, string(line))
	}

	if _, err := w.Write([]byte(strings.Join(lines, "\n"))); err != nil {
		s.logger.Error("failed to write response", "error", err)
	}
	s.logger.Info("served metadata records", "count", len(metadata))
}

// handleHealth provides a simple health check endpoint.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	hasMetaDB, err := s.storage.MetadataExists(r.Context())
	if err != nil {
		s.logger.Error("error checking metadata existence", "error", err)
		hasMetaDB = false
	}

	status := struct {
		Status        string `json:"status"`
		StorageType   string `json:"storage_type"`
		HasMetadataDB bool   `json:"has_metadata_db"`
	}{
		Status:        "ok",
		StorageType:   s.storage.Type(),
		HasMetadataDB: hasMetaDB,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(status); err != nil {
		s.logger.Error("failed to encode response", "error", err)
	}
}
