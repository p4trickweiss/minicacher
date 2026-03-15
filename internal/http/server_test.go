package webserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mockNode implements the Node interface for testing
type mockNode struct {
	data      map[string]string
	isLeader  bool
	leaderAPI string
	joinError error
}

func newMockNode() *mockNode {
	return &mockNode{
		data:     make(map[string]string),
		isLeader: true,
	}
}

func (m *mockNode) Get(key string) (string, error) {
	value, exists := m.data[key]
	if !exists {
		return "", ErrKeyNotFound
	}
	return value, nil
}

func (m *mockNode) Exists(key string) bool {
	_, exists := m.data[key]
	return exists
}

func (m *mockNode) Set(key, value string) error {
	if !m.isLeader {
		return ErrNotLeader
	}
	m.data[key] = value
	return nil
}

func (m *mockNode) Delete(key string) error {
	if !m.isLeader {
		return ErrNotLeader
	}
	delete(m.data, key)
	return nil
}

func (m *mockNode) Join(nodeID, addr string) error {
	return m.joinError
}

func (m *mockNode) IsLeader() bool {
	return m.isLeader
}

func (m *mockNode) GetLeaderAPIAddr() string {
	return m.leaderAPI
}

var (
	ErrNotLeader   = errors.New("not leader")
	ErrKeyNotFound = errors.New("key not found")
)

func TestHandleGet_Success(t *testing.T) {
	node := newMockNode()
	node.data["testkey"] = "testvalue"

	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("GET", "/store/testkey", nil)
	req.SetPathValue("key", "testkey")
	w := httptest.NewRecorder()

	server.handleGet(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["key"] != "testkey" {
		t.Errorf("Expected key 'testkey', got '%s'", resp["key"])
	}
	if resp["value"] != "testvalue" {
		t.Errorf("Expected value 'testvalue', got '%s'", resp["value"])
	}
}

func TestHandleGet_NonExistentKey(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("GET", "/store/nonexistent", nil)
	req.SetPathValue("key", "nonexistent")
	w := httptest.NewRecorder()

	server.handleGet(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["error"] != "key not found" {
		t.Errorf("Expected error 'key not found', got '%s'", resp["error"])
	}
}

func TestHandleExists_KeyExists(t *testing.T) {
	node := newMockNode()
	node.data["existingkey"] = "somevalue"
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("HEAD", "/store/existingkey", nil)
	req.SetPathValue("key", "existingkey")
	w := httptest.NewRecorder()

	server.handleExists(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestHandleExists_KeyNotFound(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("HEAD", "/store/nonexistent", nil)
	req.SetPathValue("key", "nonexistent")
	w := httptest.NewRecorder()

	server.handleExists(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleSet_Success(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	body := map[string]string{
		"key":   "newkey",
		"value": "newvalue",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/store", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleSet(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", w.Code)
	}

	// Verify data was stored
	if node.data["newkey"] != "newvalue" {
		t.Errorf("Expected value 'newvalue', got '%s'", node.data["newkey"])
	}
}

func TestHandleSet_MissingFields(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	tests := []struct {
		name string
		body map[string]string
	}{
		{
			name: "missing key",
			body: map[string]string{"value": "test"},
		},
		{
			name: "missing value",
			body: map[string]string{"key": "test"},
		},
		{
			name: "empty key",
			body: map[string]string{"key": "", "value": "test"},
		},
		{
			name: "empty value",
			body: map[string]string{"key": "test", "value": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyJSON, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/store", bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleSet(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

func TestHandleSet_InvalidJSON(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("POST", "/store", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleSet(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleDelete_Success(t *testing.T) {
	node := newMockNode()
	node.data["deletekey"] = "deletevalue"
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("DELETE", "/store/deletekey", nil)
	req.SetPathValue("key", "deletekey")
	w := httptest.NewRecorder()

	server.handleDelete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify data was deleted
	if _, exists := node.data["deletekey"]; exists {
		t.Errorf("Key should have been deleted")
	}
}

func TestHandleJoin_Success(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	body := map[string]string{
		"id":   "node2",
		"addr": "node2:12000",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/join", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleJoin(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["id"] != "node2" {
		t.Errorf("Expected id 'node2', got '%s'", resp["id"])
	}
}

func TestHandleJoin_MissingFields(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	tests := []struct {
		name string
		body map[string]string
	}{
		{
			name: "missing id",
			body: map[string]string{"addr": "node2:12000"},
		},
		{
			name: "missing addr",
			body: map[string]string{"id": "node2"},
		},
		{
			name: "empty id",
			body: map[string]string{"id": "", "addr": "node2:12000"},
		},
		{
			name: "empty addr",
			body: map[string]string{"id": "node2", "addr": ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bodyJSON, _ := json.Marshal(tt.body)
			req := httptest.NewRequest("POST", "/join", bytes.NewReader(bodyJSON))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			server.handleJoin(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("Expected status 400, got %d", w.Code)
			}
		})
	}
}

func TestHandleHealth(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	server.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%v'", resp["status"])
	}

	if resp["is_leader"] != true {
		t.Errorf("Expected is_leader true, got %v", resp["is_leader"])
	}
}

func TestProxyToLeader_WhenLeader(t *testing.T) {
	node := newMockNode()
	node.isLeader = true
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("POST", "/store", nil)
	w := httptest.NewRecorder()

	proxied := server.proxyToLeader(w, req)

	if proxied {
		t.Errorf("Expected not to proxy when node is leader")
	}
}

func TestProxyToLeader_NoLeaderAvailable(t *testing.T) {
	node := newMockNode()
	node.isLeader = false
	node.leaderAPI = "" // No leader available
	server := NewServer("localhost:8080", node, "test-node")

	req := httptest.NewRequest("POST", "/store", nil)
	w := httptest.NewRecorder()

	proxied := server.proxyToLeader(w, req)

	if !proxied {
		t.Errorf("Expected to proxy when not leader")
	}

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestHandleSet_NotLeader_NoProxy(t *testing.T) {
	// Test the handleSet behavior when not leader and proxy fails
	node := newMockNode()
	node.isLeader = false
	node.leaderAPI = "" // No leader
	server := NewServer("localhost:8080", node, "test-node")

	body := map[string]string{
		"key":   "testkey",
		"value": "testvalue",
	}
	bodyJSON, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/store", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	server.handleSet(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestWriteJSONResponse(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	w := httptest.NewRecorder()
	data := map[string]string{"message": "test"}

	server.writeJSONResponse(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", w.Header().Get("Content-Type"))
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["message"] != "test" {
		t.Errorf("Expected message 'test', got '%s'", resp["message"])
	}
}

func TestWriteJSONError(t *testing.T) {
	node := newMockNode()
	server := NewServer("localhost:8080", node, "test-node")

	w := httptest.NewRecorder()

	server.writeJSONError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["error"] != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", resp["error"])
	}
}
