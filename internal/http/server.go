// Package webserver provides an HTTP API for interacting with the distributed key-value store.
// It handles client requests and automatically forwards write operations to the Raft leader.
package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// Node defines the interface for interacting with the distributed key-value store node
type Node interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	Join(nodeID, addr string) error
	IsLeader() bool
	GetLeaderAPIAddr() string
}

// Server provides an HTTP API for the distributed key-value store
type Server struct {
	addr   string
	mux    *http.ServeMux
	server *http.Server
	node   Node
	logger *slog.Logger
}

// NewServer creates a new HTTP server instance
func NewServer(addr string, node Node, nodeID string) *Server {
	return &Server{
		addr:   addr,
		mux:    http.NewServeMux(),
		node:   node,
		logger: slog.With("component", "http_server", "node_id", nodeID),
	}
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	s.mux.HandleFunc("POST /join", s.handleJoin)
	s.mux.HandleFunc("GET /store/{key}", s.handleGet)
	s.mux.HandleFunc("POST /store", s.handleSet)
	s.mux.HandleFunc("DELETE /store/{key}", s.handleDelete)
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

// proxyToLeader forwards write requests to the Raft leader if this node is not the leader.
// Returns true if the request was proxied, false if this node is the leader.
func (s *Server) proxyToLeader(w http.ResponseWriter, r *http.Request) bool {
	if s.node.IsLeader() {
		return false
	}

	leaderAddr := s.node.GetLeaderAPIAddr()
	if leaderAddr == "" {
		s.logger.Warn("proxy failed: no leader available",
			"method", r.Method,
			"path", r.URL.Path)
		s.writeJSONError(w, http.StatusServiceUnavailable, "leader not available")
		return true
	}

	s.logger.Debug("proxying request to leader",
		"method", r.Method,
		"path", r.URL.Path,
		"leader_addr", leaderAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Error("proxy failed: error reading request body",
			"error", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to read request body")
		return true
	}

	leaderURL := fmt.Sprintf("http://%s%s", leaderAddr, r.URL.Path)
	req, err := http.NewRequest(r.Method, leaderURL, bytes.NewReader(body))
	if err != nil {
		s.logger.Error("proxy failed: error creating request to leader",
			"error", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to create proxy request")
		return true
	}

	req.Header = r.Header
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Error("proxy failed: error contacting leader",
			"leader_addr", leaderAddr,
			"error", err)
		s.writeJSONError(w, http.StatusBadGateway, "failed to contact leader")
		return true
	}
	defer resp.Body.Close()

	s.logger.Info("proxy successful",
		"method", r.Method,
		"path", r.URL.Path,
		"leader_addr", leaderAddr,
		"status", resp.StatusCode)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		s.logger.Warn("proxy warning: error copying response body",
			"error", err)
	}
	return true
}

// handleJoin handles requests to join a new node to the Raft cluster
func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   string `json:"id"`
		Addr string `json:"addr"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Warn("join failed: invalid JSON",
			"error", err)
		s.writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" || req.Addr == "" {
		s.logger.Warn("join failed: missing required fields",
			"id", req.ID,
			"addr", req.Addr)
		s.writeJSONError(w, http.StatusBadRequest, "id and addr are required")
		return
	}

	if err := s.node.Join(req.ID, req.Addr); err != nil {
		s.logger.Error("join failed",
			"node_id", req.ID,
			"addr", req.Addr,
			"error", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to join cluster")
		return
	}

	s.logger.Info("join successful",
		"node_id", req.ID,
		"addr", req.Addr)
	s.writeJSONResponse(w, http.StatusOK, map[string]string{
		"message": "node joined successfully",
		"id":      req.ID,
	})
}

// handleGet retrieves a value from the store
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	value, err := s.node.Get(key)
	if err != nil {
		s.logger.Error("GET failed",
			"key", key,
			"error", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to get value")
		return
	}

	s.logger.Debug("GET successful",
		"key", key,
		"value_len", len(value))

	s.writeJSONResponse(w, http.StatusOK, map[string]string{
		"key":   key,
		"value": value,
	})
}

// handleSet stores a key-value pair in the store
func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	if s.proxyToLeader(w, r) {
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.logger.Warn("SET failed: invalid JSON",
			"error", err)
		s.writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Key == "" || req.Value == "" {
		s.logger.Warn("SET failed: missing required fields",
			"key", req.Key,
			"value_present", req.Value != "")
		s.writeJSONError(w, http.StatusBadRequest, "key and value are required")
		return
	}

	if err := s.node.Set(req.Key, req.Value); err != nil {
		s.logger.Error("SET failed",
			"key", req.Key,
			"error", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to set value")
		return
	}

	s.logger.Info("SET successful",
		"key", req.Key,
		"value_len", len(req.Value))

	s.writeJSONResponse(w, http.StatusCreated, map[string]string{
		"message": "value set successfully",
		"key":     req.Key,
	})
}

// handleDelete removes a key from the store
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.proxyToLeader(w, r) {
		return
	}

	key := r.PathValue("key")

	if err := s.node.Delete(key); err != nil {
		s.logger.Error("DELETE failed",
			"key", key,
			"error", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to delete key")
		return
	}

	s.logger.Info("DELETE successful",
		"key", key)

	s.writeJSONResponse(w, http.StatusOK, map[string]string{
		"message": "value deleted successfully",
		"key":     key,
	})
}

// handleHealth returns the health status of the server
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	isLeader := s.node.IsLeader()

	s.writeJSONResponse(w, http.StatusOK, map[string]any{
		"status":    "healthy",
		"is_leader": isLeader,
		"time":      time.Now().Format(time.RFC3339),
	})
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.setupRoutes()

	s.server = &http.Server{
		Addr:         s.addr,
		Handler:      s.mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("starting http server",
		"addr", s.addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("shutting down http server, waiting for active connections to finish")

	err := s.server.Shutdown(ctx)
	if err != nil {
		s.logger.Error("http server shutdown error",
			"error", err)
		return err
	}

	s.logger.Info("http server stopped accepting connections")
	return nil
}

// writeJSONResponse is a helper to write JSON responses with proper headers
func (s *Server) writeJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Error("error encoding JSON response",
			"error", err)
	}
}

// writeJSONError is a helper to write JSON error responses
func (s *Server) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	s.writeJSONResponse(w, statusCode, map[string]string{
		"error": message,
	})
}
