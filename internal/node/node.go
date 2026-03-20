// Package node provides a distributed key-value store implementation using the Raft consensus algorithm.
// It ensures strong consistency across multiple nodes by replicating all operations through Raft.
package node

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb/v2"
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

// Config holds the configuration for a Node
type Config struct {
	NodeId   string
	BindAddr string
	HTTPAddr string
	DataDir  string
	Bootstrap bool
}

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)

// Node is a distributed key-value store backed by Raft consensus
type Node struct {
	cache    cache.Cache
	raft     *raft.Raft
	logger   *slog.Logger
	nodeID   string
	httpPort string
}

// New creates a new Node instance
func New() *Node {
	return &Node{
		cache:  cache.New(cache.Options{}),
		logger: slog.With("component", "node"),
	}
}

// Open initializes and starts the Raft node with the given configuration
func (n *Node) Open(config Config) error {
	if config.NodeId == "" {
		return fmt.Errorf("node ID is required")
	}
	if config.BindAddr == "" {
		return fmt.Errorf("bind address is required")
	}
	if config.DataDir == "" {
		config.DataDir = "./data"
	}

	n.nodeID = config.NodeId
	n.logger = n.logger.With("node_id", config.NodeId)

	_, httpPort, err := net.SplitHostPort(config.HTTPAddr)
	if err != nil {
		return fmt.Errorf("invalid http addr %q: %w", config.HTTPAddr, err)
	}
	n.httpPort = httpPort

	n.logger.Info("opening node",
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

	logStore, err := raftboltdb.NewBoltStore(filepath.Join(config.DataDir, "raft-log.db"))
	if err != nil {
		return fmt.Errorf("new bolt store: %w", err)
	}

	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(config.DataDir, "raft-stable.db"))
	if err != nil {
		return fmt.Errorf("new stable store: %w", err)
	}

	r, err := raft.NewRaft(
		raftConfig,
		newFSM(n),
		logStore,
		stableStore,
		snapshots,
		transport,
	)
	if err != nil {
		return fmt.Errorf("new Raft: %s", err)
	}
	n.raft = r
	n.logger.Info("raft instance created successfully")

	// Bootstrap cluster if this is the first node
	if config.Bootstrap {
		n.logger.Info("bootstrapping new cluster")
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
			n.logger.Warn("bootstrap cluster failed (may be expected if cluster already exists)",
				"error", err)
		} else {
			n.logger.Info("cluster bootstrapped successfully")
		}
	}

	return nil
}

// Join adds a node to the Raft cluster. This should be called on the leader.
func (n *Node) Join(nodeId, addr string) error {
	n.logger.Info("received join request",
		"joining_node_id", nodeId,
		"joining_node_addr", addr,
		"current_state", n.raft.State().String())

	configFuture := n.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		n.logger.Error("failed to get raft configuration",
			"error", err)
		return err
	}

	// Check if node already exists and handle conflicts
	if err := n.handleExistingServer(nodeId, addr, configFuture.Configuration().Servers); err != nil {
		return err
	}

	// Add the new voter
	n.logger.Info("adding voter to cluster",
		"joining_node_id", nodeId,
		"joining_node_addr", addr)
	f := n.raft.AddVoter(raft.ServerID(nodeId), raft.ServerAddress(addr), 0, 0)
	if err := f.Error(); err != nil {
		n.logger.Error("failed to add voter",
			"joining_node_id", nodeId,
			"error", err)
		return fmt.Errorf("failed to add voter: %w", err)
	}

	n.logger.Info("node successfully joined cluster",
		"joined_node_id", nodeId)
	return nil
}

// handleExistingServer removes any conflicting server configurations before adding a new server
func (n *Node) handleExistingServer(nodeId, addr string, servers []raft.Server) error {
	for _, srv := range servers {
		// Check for ID or address conflicts
		if srv.ID == raft.ServerID(nodeId) || srv.Address == raft.ServerAddress(addr) {
			// If both ID and address match, node is already a member
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeId) {
				n.logger.Info("node already member of cluster, ignoring join request",
					"node_id", nodeId,
					"addr", addr)
				return nil
			}

			// Remove conflicting server configuration
			n.logger.Warn("removing existing conflicting server before adding new one",
				"existing_server_id", srv.ID,
				"existing_server_addr", srv.Address,
				"new_node_id", nodeId,
				"new_node_addr", addr)
			future := n.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				n.logger.Error("failed to remove existing node",
					"node_id", nodeId,
					"error", err)
				return fmt.Errorf("failed to remove existing node %s at %s: %w", nodeId, addr, err)
			}
		}
	}
	return nil
}

// IsLeader returns true if this node is the current Raft leader
func (n *Node) IsLeader() bool {
	return n.raft.State() == raft.Leader
}

// GetLeaderAPIAddr returns the API address of the current cluster leader
func (n *Node) GetLeaderAPIAddr() string {
	addr, _ := n.raft.LeaderWithID()
	return n.raftToAPIAddr(string(addr))
}

// raftToAPIAddr converts a Raft address to the corresponding API address
// by replacing the Raft port with the node's configured HTTP port.
func (n *Node) raftToAPIAddr(raftAddr string) string {
	host, _, err := net.SplitHostPort(raftAddr)
	if err != nil {
		return ""
	}
	return net.JoinHostPort(host, n.httpPort)
}

// Get retrieves a value from the key-value store
func (n *Node) Get(key string) (string, error) {
	value, exists := n.cache.Get(key)
	if !exists {
		return "", fmt.Errorf("key not found")
	}
	return value, nil
}

// Exists checks if a key exists in the store
func (n *Node) Exists(key string) bool {
	return n.cache.Exists(key)
}

// Set stores a key-value pair. Must be called on the leader.
func (n *Node) Set(key, value string) error {
	state := n.raft.State()
	if state != raft.Leader {
		n.logger.Debug("set rejected: not leader",
			"key", key,
			"current_state", state.String())
		return fmt.Errorf("not leader")
	}

	n.logger.Debug("applying set operation",
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

	future := n.raft.Apply(b, raftTimeout)
	return future.Error()
}

// Delete removes a key from the store. Must be called on the leader.
func (n *Node) Delete(key string) error {
	state := n.raft.State()
	if state != raft.Leader {
		n.logger.Debug("delete rejected: not leader",
			"key", key,
			"current_state", state.String())
		return fmt.Errorf("not leader")
	}

	n.logger.Debug("applying delete operation",
		"key", key)

	cmd := &command{
		Op:  OpDelete,
		Key: key,
	}

	b, err := cmd.Encode()
	if err != nil {
		return err
	}

	future := n.raft.Apply(b, raftTimeout)
	return future.Error()
}

// applySet is called by the FSM to apply a Set command to the key-value store
func (n *Node) applySet(key, value string) any {
	n.cache.Set(key, value)
	n.logger.Debug("applied set to state machine",
		"key", key,
		"value_len", len(value))
	return nil
}

// applyDelete is called by the FSM to apply a Delete command to the key-value store
func (n *Node) applyDelete(key string) any {
	n.cache.Delete(key)
	n.logger.Debug("applied delete to state machine",
		"key", key)
	return nil
}

// Close gracefully shuts down the Raft instance
func (n *Node) Close() error {
	if n.raft == nil {
		return nil
	}

	n.logger.Info("shutting down raft instance")

	// Shutdown triggers a snapshot and stops the Raft instance
	future := n.raft.Shutdown()
	if err := future.Error(); err != nil {
		n.logger.Error("error during raft shutdown",
			"error", err)
		return fmt.Errorf("raft shutdown failed: %w", err)
	}

	n.logger.Info("raft shutdown complete")
	return nil
}
