package store

import (
	"testing"
)

func TestNew(t *testing.T) {
	s := New()

	if s == nil {
		t.Fatal("New() returned nil")
	}

	if s.cache == nil {
		t.Error("Cache is nil")
	}

	if s.logger == nil {
		t.Error("Logger is nil")
	}
}

func TestRaftToAPIAddr(t *testing.T) {
	tests := []struct {
		name     string
		raftAddr string
		want     string
	}{
		{
			name:     "valid address",
			raftAddr: "node1:12000",
			want:     "node1:11000",
		},
		{
			name:     "localhost address",
			raftAddr: "localhost:12000",
			want:     "localhost:11000",
		},
		{
			name:     "ip address",
			raftAddr: "192.168.1.1:12000",
			want:     "192.168.1.1:11000",
		},
		{
			name:     "invalid address",
			raftAddr: "invalid",
			want:     "",
		},
		{
			name:     "empty address",
			raftAddr: "",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := raftToAPIAddr(tt.raftAddr)
			if got != tt.want {
				t.Errorf("raftToAPIAddr(%q) = %q, want %q", tt.raftAddr, got, tt.want)
			}
		})
	}
}

func TestApplySet(t *testing.T) {
	s := New()

	// Apply a set operation
	s.applySet("key1", "value1")

	// Verify it was stored in cache
	val, err := s.Get("key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if val != "value1" {
		t.Errorf("Get(key1) = %q, want %q", val, "value1")
	}
}

func TestApplyDelete(t *testing.T) {
	s := New()

	// First set a value
	s.applySet("key1", "value1")

	// Verify it exists
	val, _ := s.Get("key1")
	if val != "value1" {
		t.Errorf("Setup failed: expected value1, got %q", val)
	}

	// Apply delete
	s.applyDelete("key1")

	// Verify it was deleted
	val, _ = s.Get("key1")
	if val != "" {
		t.Errorf("Get after delete should return empty string, got %q", val)
	}
}

func TestGet(t *testing.T) {
	s := New()

	// Test getting non-existent key
	val, err := s.Get("nonexistent")
	if err != nil {
		t.Errorf("Get should not error on non-existent key: %v", err)
	}
	if val != "" {
		t.Errorf("Get(nonexistent) = %q, want empty string", val)
	}

	// Set and get a value
	s.applySet("testkey", "testvalue")
	val, err = s.Get("testkey")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if val != "testvalue" {
		t.Errorf("Get(testkey) = %q, want %q", val, "testvalue")
	}
}

func TestCommandEncodeDecode(t *testing.T) {
	tests := []struct {
		name string
		cmd  command
	}{
		{
			name: "set command",
			cmd: command{
				Op:    OpSet,
				Key:   "key1",
				Value: "value1",
			},
		},
		{
			name: "delete command",
			cmd: command{
				Op:  OpDelete,
				Key: "key1",
			},
		},
		{
			name: "set with empty value",
			cmd: command{
				Op:    OpSet,
				Key:   "key1",
				Value: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode
			data, err := tt.cmd.Encode()
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			// Decode
			var decoded command
			if err := decoded.Decode(data); err != nil {
				t.Fatalf("Decode failed: %v", err)
			}

			// Verify
			if decoded.Op != tt.cmd.Op {
				t.Errorf("Op = %v, want %v", decoded.Op, tt.cmd.Op)
			}
			if decoded.Key != tt.cmd.Key {
				t.Errorf("Key = %q, want %q", decoded.Key, tt.cmd.Key)
			}
			if decoded.Value != tt.cmd.Value {
				t.Errorf("Value = %q, want %q", decoded.Value, tt.cmd.Value)
			}
		})
	}
}

func TestCommandDecodeInvalid(t *testing.T) {
	var cmd command
	err := cmd.Decode([]byte("invalid json"))
	if err == nil {
		t.Error("Expected error decoding invalid JSON, got nil")
	}
}

func TestClose_WithoutOpen(t *testing.T) {
	s := New()

	// Close without opening should not error
	err := s.Close()
	if err != nil {
		t.Errorf("Close without Open should not error: %v", err)
	}
}

func TestMultipleOperations(t *testing.T) {
	s := New()

	// Perform multiple operations
	operations := []struct {
		key   string
		value string
	}{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
	}

	// Set all values
	for _, op := range operations {
		s.applySet(op.key, op.value)
	}

	// Verify all values
	for _, op := range operations {
		val, err := s.Get(op.key)
		if err != nil {
			t.Errorf("Get(%q) failed: %v", op.key, err)
		}
		if val != op.value {
			t.Errorf("Get(%q) = %q, want %q", op.key, val, op.value)
		}
	}

	// Delete one value
	s.applyDelete("key2")

	// Verify deletion
	val, _ := s.Get("key2")
	if val != "" {
		t.Errorf("Get(key2) after delete = %q, want empty string", val)
	}

	// Verify others still exist
	val, _ = s.Get("key1")
	if val != "value1" {
		t.Errorf("Get(key1) = %q, want value1", val)
	}
	val, _ = s.Get("key3")
	if val != "value3" {
		t.Errorf("Get(key3) = %q, want value3", val)
	}
}

func TestUpdateValue(t *testing.T) {
	s := New()

	// Set initial value
	s.applySet("key1", "initial")
	val, _ := s.Get("key1")
	if val != "initial" {
		t.Errorf("Initial value = %q, want initial", val)
	}

	// Update value
	s.applySet("key1", "updated")
	val, _ = s.Get("key1")
	if val != "updated" {
		t.Errorf("Updated value = %q, want updated", val)
	}
}
