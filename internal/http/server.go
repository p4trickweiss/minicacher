package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	Join(nodeID, addr string) error
	IsLeader() bool
	GetLeaderAddr() string
}

type Server struct {
	addr   string
	mux    *http.ServeMux
	server *http.Server

	store Store
}

func NewServer(addr string, store Store) *Server {
	return &Server{
		addr:  addr,
		mux:   http.NewServeMux(),
		store: store,
	}
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("POST /join", s.handleJoin)
	s.mux.HandleFunc("GET /store/{key}", s.handleGet)
	s.mux.HandleFunc("POST /store", s.handleSet)
	s.mux.HandleFunc("DELETE /store/{key}", s.handleDelete)
	s.mux.HandleFunc("GET /health", s.handleHealth)
}

func (s *Server) proxyToLeader(w http.ResponseWriter, r *http.Request) bool {
	if s.store.IsLeader() {
		return false
	}

	leaderAddr := s.store.GetLeaderAddr()
	if leaderAddr == "" {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "leader not available",
		})
		return true
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return true
	}

	leaderURL := fmt.Sprintf("http://%s%s", leaderAddr, r.URL.Path)
	req, err := http.NewRequest(r.Method, leaderURL, bytes.NewReader(body))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return true
	}

	req.Header = r.Header
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to contact leader",
		})
		return true
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
	return true
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	m := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if len(m) != 2 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	remoteAddr, ok := m["addr"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	nodeId, ok := m["id"]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := s.store.Join(nodeId, remoteAddr); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")

	value, err := s.store.Get(key)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "key not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"key":   key,
		"value": value,
	})
}

func (s *Server) handleSet(w http.ResponseWriter, r *http.Request) {
	if s.proxyToLeader(w, r) {
		return
	}

	m := map[string]string{}
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "invalid request body",
		})
		return
	}

	key, okKey := m["key"]
	value, okValue := m["value"]

	if !okKey || !okValue {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "key and value are required",
		})
		return
	}

	if err := s.store.Set(key, value); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "failed to set value",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "value set successfully",
		"key":     key,
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if s.proxyToLeader(w, r) {
		return
	}

	key := r.PathValue("key")

	if err := s.store.Delete(key); err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "key not found",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "value deleted successfully",
		"key":     key,
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (s *Server) Start() error {
	s.setupRoutes()

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: s.mux,
	}

	log.Printf("Starting http server on %s", s.addr)
	return s.server.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
