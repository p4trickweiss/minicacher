// Package store provides a distributed key-value store implementation using the Raft consensus algorithm.
// It ensures strong consistency across multiple nodes by replicating all operations through Raft.
package store

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	"github.com/p4trickweiss/distributed-cache/internal/cache"
)

// Command operation types
type commandType uint8

const (
	OpSet commandType = iota
	OpDelete
)

// command represents a state machine command that will be replicated via Raft
type command struct {
	Op    commandType
	Key   string
	Value string
}

func (c *command) Encode() ([]byte, error) {
	return json.Marshal(c)
}

func (c *command) Decode(b []byte) error {
	return json.Unmarshal(b, c)
}

// Config holds the configuration for a Store node
type Config struct {
	NodeId    string
	BindAddr  string
	DataDir   string
	Bootstrap bool
}

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)

// Store is a distributed key-value store backed by Raft consensus
type Store struct {
	cache  cache.Cache
	raft   *raft.Raft
	logger *slog.Logger
	nodeID string
}

// New creates a new Store instance
func New() *Store {
	return &Store{
		cache:  cache.New(cache.Options{}),
		logger: slog.With("component", "store"),
	}
}

// Open initializes and starts the Raft node with the given configuration
func (s *Store) Open(config Config) error {
	if config.NodeId == "" {
		return fmt.Errorf("node ID is required")
	}
	if config.BindAddr == "" {
		return fmt.Errorf("bind address is required")
	}
	if config.DataDir == "" {
		config.DataDir = "./data"
	}

	s.nodeID = config.NodeId
	s.logger = s.logger.With("node_id", config.NodeId)

	s.logger.Info("opening store",
		"raft_addr", config.BindAddr,
		"data_dir", config.DataDir,
		"bootstrap", config.Bootstrap)

	raftConfig := raft.DefaultConfig()
	raftConfig.LocalID = raft.ServerID(config.NodeId)

	addr, err := net.ResolveTCPAddr("tcp", config.BindAddr)
	if err != nil {
		return err
	}
	transport, err := raft.NewTCPTransport(config.BindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return err
	}

	snapshots, err := raft.NewFileSnapshotStore(
		filepath.Join(config.DataDir),
		retainSnapshotCount,
		os.Stderr,
	)
	if err != nil {
		return fmt.Errorf("file snaphot store: %s", err)
	}

	var logStore raft.LogStore
	var stableStore raft.StableStore
	logStore = raft.NewInmemStore()
	stableStore = raft.NewInmemStore()

	r, err := raft.NewRaft(
		raftConfig,
		newFSM(s),
		logStore,
		stableStore,
		snapshots,
		transport,
	)
	if err != nil {
		return fmt.Errorf("new Raft: %s", err)
	}
	s.raft = r
	s.logger.Info("raft instance created successfully")

	// Bootstrap cluster if this is the first node
	if config.Bootstrap {
		s.logger.Info("bootstrapping new cluster")
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftConfig.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}

		future := r.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			// It's okay if bootstrap fails because cluster already exists
			s.logger.Warn("bootstrap cluster failed (may be expected if cluster already exists)",
				"error", err)
		} else {
			s.logger.Info("cluster bootstrapped successfully")
		}
	}

	return nil
}

// Join adds a node to the Raft cluster. This should be called on the leader.
func (s *Store) Join(nodeId, addr string) error {
	s.logger.Info("received join request",
		"joining_node_id", nodeId,
		"joining_node_addr", addr,
		"current_state", s.raft.State().String())

	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Error("failed to get raft configuration",
			"error", err)
		return err
	}

	// Check if node already exists and handle conflicts
	if err := s.handleExistingServer(nodeId, addr, configFuture.Configuration().Servers); err != nil {
		return err
	}

	// Add the new voter
	s.logger.Info("adding voter to cluster",
		"joining_node_id", nodeId,
		"joining_node_addr", addr)
	f := s.raft.AddVoter(raft.ServerID(nodeId), raft.ServerAddress(addr), 0, 0)
	if err := f.Error(); err != nil {
		s.logger.Error("failed to add voter",
			"joining_node_id", nodeId,
			"error", err)
		return fmt.Errorf("failed to add voter: %w", err)
	}

	s.logger.Info("node successfully joined cluster",
		"joined_node_id", nodeId)
	return nil
}

// handleExistingServer removes any conflicting server configurations before adding a new server
func (s *Store) handleExistingServer(nodeId, addr string, servers []raft.Server) error {
	for _, srv := range servers {
		// Check for ID or address conflicts
		if srv.ID == raft.ServerID(nodeId) || srv.Address == raft.ServerAddress(addr) {
			// If both ID and address match, node is already a member
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeId) {
				s.logger.Info("node already member of cluster, ignoring join request",
					"node_id", nodeId,
					"addr", addr)
				return nil
			}

			// Remove conflicting server configuration
			s.logger.Warn("removing existing conflicting server before adding new one",
				"existing_server_id", srv.ID,
				"existing_server_addr", srv.Address,
				"new_node_id", nodeId,
				"new_node_addr", addr)
			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				s.logger.Error("failed to remove existing node",
					"node_id", nodeId,
					"error", err)
				return fmt.Errorf("failed to remove existing node %s at %s: %w", nodeId, addr, err)
			}
		}
	}
	return nil
}

// IsLeader returns true if this node is the current Raft leader
func (s *Store) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

// GetLeaderAPIAddr returns the API address of the current cluster leader
func (s *Store) GetLeaderAPIAddr() string {
	addr, _ := s.raft.LeaderWithID()
	return raftToAPIAddr(string(addr))
}

// raftToAPIAddr converts a Raft address to the corresponding API address
func raftToAPIAddr(raftAddr string) string {
	host, _, err := net.SplitHostPort(raftAddr)
	if err != nil {
		return ""
	}

	httpPort := 11000
	return fmt.Sprintf("%s:%d", host, httpPort)
}

// Get retrieves a value from the key-value store
func (s *Store) Get(key string) (string, error) {
	return s.cache.Get(key), nil
}

// Set stores a key-value pair. Must be called on the leader.
func (s *Store) Set(key, value string) error {
	state := s.raft.State()
	if state != raft.Leader {
		s.logger.Debug("set rejected: not leader",
			"key", key,
			"current_state", state.String())
		return fmt.Errorf("not leader")
	}

	s.logger.Debug("applying set operation",
		"key", key,
		"value_len", len(value))

	cmd := &command{
		Op:    OpSet,
		Key:   key,
		Value: value,
	}

	b, err := cmd.Encode()
	if err != nil {
		return err
	}

	future := s.raft.Apply(b, raftTimeout)
	return future.Error()
}

// Delete removes a key from the store. Must be called on the leader.
func (s *Store) Delete(key string) error {
	state := s.raft.State()
	if state != raft.Leader {
		s.logger.Debug("delete rejected: not leader",
			"key", key,
			"current_state", state.String())
		return fmt.Errorf("not leader")
	}

	s.logger.Debug("applying delete operation",
		"key", key)

	cmd := &command{
		Op:  OpDelete,
		Key: key,
	}

	b, err := cmd.Encode()
	if err != nil {
		return err
	}

	future := s.raft.Apply(b, raftTimeout)
	return future.Error()
}

// applySet is called by the FSM to apply a Set command to the key-value store
func (s *Store) applySet(key, value string) any {
	s.cache.Set(key, value)
	s.logger.Debug("applied set to state machine",
		"key", key,
		"value_len", len(value))
	return nil
}

// applyDelete is called by the FSM to apply a Delete command to the key-value store
func (s *Store) applyDelete(key string) any {
	s.cache.Delete(key)
	s.logger.Debug("applied delete to state machine",
		"key", key)
	return nil
}

// Close gracefully shuts down the Raft instance
func (s *Store) Close() error {
	if s.raft == nil {
		return nil
	}

	s.logger.Info("shutting down raft instance")

	// Shutdown triggers a snapshot and stops the Raft instance
	future := s.raft.Shutdown()
	if err := future.Error(); err != nil {
		s.logger.Error("error during raft shutdown",
			"error", err)
		return fmt.Errorf("raft shutdown failed: %w", err)
	}

	s.logger.Info("raft shutdown complete")
	return nil
}
