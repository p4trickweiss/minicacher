package node

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/raft"
)

// fsm implements the raft.FSM interface, providing the finite state machine
// that applies committed log entries to the key-value store.
type fsm struct {
	node *Node
}

// newFSM creates a new finite state machine backed by the given Node
func newFSM(n *Node) *fsm {
	return &fsm{node: n}
}

// Apply is called by Raft when a log entry is committed.
// It decodes the command and applies it to the state machine.
func (f *fsm) Apply(log *raft.Log) any {
	var cmd command
	if err := cmd.Decode(log.Data); err != nil {
		return err
	}

	switch cmd.Op {
	case OpSet:
		return f.node.applySet(cmd.Key, cmd.Value)
	case OpDelete:
		return f.node.applyDelete(cmd.Key)
	default:
		return fmt.Errorf("Unknown op")
	}
}

// Snapshot creates a point-in-time snapshot of the current state.
// This is called by Raft to create snapshots for log compaction.
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	snapshot := f.node.cache.Snapshot()
	return &fsmSnapshot{store: snapshot}, nil
}

// Restore restores the state machine from a snapshot.
// This is called when a node is catching up or recovering.
func (f *fsm) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	var kv map[string]string
	if err := json.NewDecoder(rc).Decode(&kv); err != nil {
		return err
	}

	f.node.cache.Restore(kv)
	return nil
}

// fsmSnapshot represents a point-in-time snapshot of the key-value store.
// It implements the raft.FSMSnapshot interface.
type fsmSnapshot struct {
	store map[string]string
}

// Persist writes the snapshot to the provided sink.
// If an error occurs, it cancels the snapshot.
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	if err := json.NewEncoder(sink).Encode(s.store); err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	if err := sink.Close(); err != nil {
		return fmt.Errorf("failed to close snapshot sink: %w", err)
	}

	return nil
}

// Release is called when Raft is done with the snapshot.
// Since we're using a simple map clone, there's no cleanup needed.
func (s *fsmSnapshot) Release() {
	// No resources to release for in-memory snapshot
}
