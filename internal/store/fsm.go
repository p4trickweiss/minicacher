package store

import (
	"encoding/json"
	"fmt"
	"io"
	"maps"

	"github.com/hashicorp/raft"
)

type fsm struct {
	store *Store
}

func newFSM(s *Store) *fsm {
	return &fsm{store: s}
}

func (f *fsm) Apply(log *raft.Log) any {
	var cmd command
	if err := cmd.Decode(log.Data); err != nil {
		return err
	}

	switch cmd.Op {
	case OpSet:
		return f.store.applySet(cmd.Key, cmd.Value)
	case OpDelete:
		return f.store.applyDelete(cmd.Key)
	default:
		return fmt.Errorf("Unknown op")
	}
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	f.store.mu.Lock()
	defer f.store.mu.Unlock()

	clone := maps.Clone(f.store.kv)
	return &fsmSnapshot{store: clone}, nil
}

func (f *fsm) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	var kv map[string]string
	if err := json.NewDecoder(rc).Decode(&kv); err != nil {
		return err
	}

	f.store.mu.Lock()
	defer f.store.mu.Unlock()
	f.store.kv = kv
	return nil
}

type fsmSnapshot struct {
	store map[string]string
}

func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := json.NewEncoder(sink).Encode(s.store)
	if err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}

func (s *fsmSnapshot) Release() {}
