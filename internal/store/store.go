package store

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

type commandType uint8

const (
	OpSet commandType = iota
	OpDelete
)

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

type Store struct {
	mu     sync.Mutex
	kv     map[string]string
	raft   *raft.Raft
	logger *log.Logger
}

func New() *Store {
	return &Store{
		kv:     make(map[string]string),
		logger: log.New(os.Stderr, "[store]", log.LstdFlags),
	}
}

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

	if config.Bootstrap {
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
			s.logger.Printf("bootstrap cluster: %v (this may be expected if cluster already exists)", err)
		}
	}

	return nil
}

func (s *Store) Join(nodeId, addr string) error {
	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		s.logger.Printf("failed to get raft configuration: %v", err)
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeId) || srv.Address == raft.ServerAddress(addr) {
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeId) {
				s.logger.Printf("node %s at %s already member of cluster, ignoring join request", nodeId, addr)
				return nil
			}

			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("error removing existing node %s at %s: %s", nodeId, addr, err)
			}
		}
	}

	f := s.raft.AddVoter(raft.ServerID(nodeId), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}

	return nil
}

func (s *Store) IsLeader() bool {
	return s.raft.State() == raft.Leader
}

func (s *Store) GetLeaderAddr() string {
	addr, _ := s.raft.LeaderWithID()
	return raftToAPIAddr(string(addr))
}

func raftToAPIAddr(raftAddr string) string {
	host, _, err := net.SplitHostPort(raftAddr)
	if err != nil {
		return ""
	}

	httpPort := 11000
	return fmt.Sprintf("%s:%d", host, httpPort)
}

func (s *Store) Get(key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.kv[key], nil
}

func (s *Store) Set(key, value string) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("Not leader")
	}

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

func (s *Store) Delete(key string) error {
	if s.raft.State() != raft.Leader {
		return fmt.Errorf("Not leader")
	}

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

func (s *Store) applySet(key, value string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kv[key] = value
	return nil
}

func (s *Store) applyDelete(key string) any {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.kv, key)
	return nil
}
