// Package webserver provides an HTTP API for interacting with the distributed key-value store.
// It handles client requests and automatically forwards write operations to the Raft leader.
package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Store defines the interface for interacting with the distributed key-value store
type Store interface {
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
	store  Store
	logger *log.Logger
}

// NewServer creates a new HTTP server instance
func NewServer(addr string, store Store) *Server {
	return &Server{
		addr:   addr,
		mux:    http.NewServeMux(),
		store:  store,
		logger: log.New(os.Stderr, "[webserver]", log.LstdFlags),
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
	if s.store.IsLeader() {
		return false
	}

	leaderAddr := s.store.GetLeaderAPIAddr()
	if leaderAddr == "" {
		s.logger.Printf("proxy failed: no leader available for %s %s", r.Method, r.URL.Path)
		s.writeJSONError(w, http.StatusServiceUnavailable, "leader not available")
		return true
	}

	s.logger.Printf("proxying %s %s to leader at %s", r.Method, r.URL.Path, leaderAddr)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.logger.Printf("proxy failed: error reading request body: %v", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to read request body")
		return true
	}

	leaderURL := fmt.Sprintf("http://%s%s", leaderAddr, r.URL.Path)
	req, err := http.NewRequest(r.Method, leaderURL, bytes.NewReader(body))
	if err != nil {
		s.logger.Printf("proxy failed: error creating request to leader: %v", err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to create proxy request")
		return true
	}

	req.Header = r.Header
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		s.logger.Printf("proxy failed: error contacting leader at %s: %v", leaderAddr, err)
		s.writeJSONError(w, http.StatusBadGateway, "failed to contact leader")
		return true
	}
	defer resp.Body.Close()

	s.logger.Printf("proxy successful: %s %s -> leader (status: %d)", r.Method, r.URL.Path, resp.StatusCode)
	w.WriteHeader(resp.StatusCode)
	if _, err := io.Copy(w, resp.Body); err != nil {
		s.logger.Printf("proxy warning: error copying response body: %v", err)
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
		s.logger.Printf("join failed: invalid JSON: %v", err)
		s.writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ID == "" || req.Addr == "" {
		s.logger.Printf("join failed: missing required fields (id=%s, addr=%s)", req.ID, req.Addr)
		s.writeJSONError(w, http.StatusBadRequest, "id and addr are required")
		return
	}

	if err := s.store.Join(req.ID, req.Addr); err != nil {
		s.logger.Printf("join failed: error adding node %s at %s: %v", req.ID, req.Addr, err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to join cluster")
		return
	}

	s.logger.Printf("join successful: node %s at %s", req.ID, req.Addr)
	s.writeJSONResponse(w, http.StatusOK, map[string]string{
		"message": "node joined successfully",
		"id":      req.ID,
	})
}

// handleGet retrieves a value from the store
func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	value, err := s.store.Get(key)
	if err != nil {
		s.logger.Printf("GET failed: key=%s, error=%v", key, err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to get value")
		return
	}

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
		s.logger.Printf("SET failed: invalid JSON: %v", err)
		s.writeJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Key == "" || req.Value == "" {
		s.logger.Printf("SET failed: missing required fields (key=%s, value_present=%t)",
			req.Key, req.Value != "")
		s.writeJSONError(w, http.StatusBadRequest, "key and value are required")
		return
	}

	if err := s.store.Set(req.Key, req.Value); err != nil {
		s.logger.Printf("SET failed: key=%s, error=%v", req.Key, err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to set value")
		return
	}

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

	if err := s.store.Delete(key); err != nil {
		s.logger.Printf("DELETE failed: key=%s, error=%v", key, err)
		s.writeJSONError(w, http.StatusInternalServerError, "failed to delete key")
		return
	}

	s.writeJSONResponse(w, http.StatusOK, map[string]string{
		"message": "value deleted successfully",
		"key":     key,
	})
}

// handleHealth returns the health status of the server
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	isLeader := s.store.IsLeader()

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

	log.Printf("Starting http server on %s", s.addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// writeJSONResponse is a helper to write JSON responses with proper headers
func (s *Server) writeJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		s.logger.Printf("error encoding JSON response: %v", err)
	}
}

// writeJSONError is a helper to write JSON error responses
func (s *Server) writeJSONError(w http.ResponseWriter, statusCode int, message string) {
	s.writeJSONResponse(w, statusCode, map[string]string{
		"error": message,
	})
}
