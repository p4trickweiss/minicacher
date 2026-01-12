package webserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type Store interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Delete(key string) error
	Join(nodeID string, addr string) error
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
	s.mux.HandleFunc("/join", s.handleJoin)
	s.mux.HandleFunc("/health", s.handleHealth)
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
