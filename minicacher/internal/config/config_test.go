package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	// Create a temporary empty config file
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	content := `
node:
  data_dir: "./data"
`
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults
	if cfg.HTTPAddr() != "localhost:11000" {
		t.Errorf("HTTPAddr() = %s, want localhost:11000", cfg.HTTPAddr())
	}
	if cfg.RaftAddr() != "localhost:12000" {
		t.Errorf("RaftAddr() = %s, want localhost:12000", cfg.RaftAddr())
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("Logging.Level = %s, want info", cfg.Logging.Level)
	}
	if cfg.Logging.JSON != false {
		t.Errorf("Logging.JSON = %v, want false", cfg.Logging.JSON)
	}
	if cfg.Cluster.JoinAddr != "" {
		t.Errorf("Cluster.JoinAddr = %s, want empty", cfg.Cluster.JoinAddr)
	}
}

func TestLoad_CustomConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	content := `
node:
  id: "test-node"
  bind_addr: "localhost"
  data_dir: "/tmp/test-data"

http:
  port: 8080

raft:
  port: 9090

cluster:
  join_addr: "localhost:7070"

logging:
  level: "debug"
  json: true
`
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check custom values
	if cfg.Node.ID != "test-node" {
		t.Errorf("Node.ID = %s, want test-node", cfg.Node.ID)
	}
	if cfg.Node.DataDir != "/tmp/test-data" {
		t.Errorf("Node.DataDir = %s, want /tmp/test-data", cfg.Node.DataDir)
	}
	if cfg.HTTPAddr() != "localhost:8080" {
		t.Errorf("HTTPAddr() = %s, want localhost:8080", cfg.HTTPAddr())
	}
	if cfg.RaftAddr() != "localhost:9090" {
		t.Errorf("RaftAddr() = %s, want localhost:9090", cfg.RaftAddr())
	}
	if cfg.Cluster.JoinAddr != "localhost:7070" {
		t.Errorf("Cluster.JoinAddr = %s, want localhost:7070", cfg.Cluster.JoinAddr)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %s, want debug", cfg.Logging.Level)
	}
	if cfg.Logging.JSON != true {
		t.Errorf("Logging.JSON = %v, want true", cfg.Logging.JSON)
	}
}

func TestLoad_EnvironmentVariables(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	content := `
node:
  id: "config-node"
  bind_addr: "localhost"
  data_dir: "./data"

http:
  port: 11000

logging:
  level: "info"
`
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create config file: %v", err)
	}

	// Set environment variables
	os.Setenv("DCACHE_NODE_ID", "env-node")
	os.Setenv("DCACHE_HTTP_PORT", "9999")
	os.Setenv("DCACHE_LOGGING_LEVEL", "debug")
	defer func() {
		os.Unsetenv("DCACHE_NODE_ID")
		os.Unsetenv("DCACHE_HTTP_PORT")
		os.Unsetenv("DCACHE_LOGGING_LEVEL")
	}()

	cfg, err := Load(configFile)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Environment variables should override config file
	if cfg.Node.ID != "env-node" {
		t.Errorf("Node.ID = %s, want env-node", cfg.Node.ID)
	}
	if cfg.HTTP.Port != 9999 {
		t.Errorf("HTTP.Port = %d, want 9999", cfg.HTTP.Port)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("Logging.Level = %s, want debug", cfg.Logging.Level)
	}
}

func TestLoad_NoConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	nonExistentFile := filepath.Join(tmpDir, "nonexistent.yaml")

	cfg, err := Load(nonExistentFile)
	if err != nil {
		t.Fatalf("Load() should not error when config file not found, got: %v", err)
	}

	// Should use defaults
	if cfg.HTTPAddr() != "localhost:11000" {
		t.Errorf("HTTPAddr() = %s, want localhost:11000", cfg.HTTPAddr())
	}
}

func TestValidate_MissingBindAddr(t *testing.T) {
	cfg := &Config{
		Node:    NodeConfig{BindAddr: "", DataDir: "./data"},
		HTTP:    HTTPConfig{Port: 11000},
		Raft:    RaftConfig{Port: 12000},
		Logging: LoggingConfig{Level: "info"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when bind_addr is empty")
	}
	if err.Error() != "node.bind_addr is required" {
		t.Errorf("Validate() error = %v, want 'node.bind_addr is required'", err)
	}
}

func TestValidate_MissingDataDir(t *testing.T) {
	cfg := &Config{
		Node:    NodeConfig{BindAddr: "localhost", DataDir: ""},
		HTTP:    HTTPConfig{Port: 11000},
		Raft:    RaftConfig{Port: 12000},
		Logging: LoggingConfig{Level: "info"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when data_dir is empty")
	}
	if err.Error() != "node.data_dir is required" {
		t.Errorf("Validate() error = %v, want 'node.data_dir is required'", err)
	}
}

func TestValidate_InvalidHTTPPort(t *testing.T) {
	cfg := &Config{
		Node:    NodeConfig{BindAddr: "localhost", DataDir: "./data"},
		HTTP:    HTTPConfig{Port: 0},
		Raft:    RaftConfig{Port: 12000},
		Logging: LoggingConfig{Level: "info"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when http.port is 0")
	}
	if err.Error() != "http.port must be a positive integer" {
		t.Errorf("Validate() error = %v, want 'http.port must be a positive integer'", err)
	}
}

func TestValidate_InvalidRaftPort(t *testing.T) {
	cfg := &Config{
		Node:    NodeConfig{BindAddr: "localhost", DataDir: "./data"},
		HTTP:    HTTPConfig{Port: 11000},
		Raft:    RaftConfig{Port: 0},
		Logging: LoggingConfig{Level: "info"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when raft.port is 0")
	}
	if err.Error() != "raft.port must be a positive integer" {
		t.Errorf("Validate() error = %v, want 'raft.port must be a positive integer'", err)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := &Config{
		Node:    NodeConfig{BindAddr: "localhost", DataDir: "./data"},
		HTTP:    HTTPConfig{Port: 11000},
		Raft:    RaftConfig{Port: 12000},
		Logging: LoggingConfig{Level: "invalid"},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should error when log level is invalid")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := &Config{
		Node:    NodeConfig{ID: "node1", BindAddr: "localhost", DataDir: "./data"},
		HTTP:    HTTPConfig{Port: 11000},
		Raft:    RaftConfig{Port: 12000},
		Cluster: ClusterConfig{JoinAddr: ""},
		Logging: LoggingConfig{Level: "info", JSON: false},
	}

	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() should not error for valid config, got: %v", err)
	}
}

func TestIsBootstrap(t *testing.T) {
	tests := []struct {
		name     string
		joinAddr string
		want     bool
	}{
		{
			name:     "empty join address is bootstrap",
			joinAddr: "",
			want:     true,
		},
		{
			name:     "non-empty join address is not bootstrap",
			joinAddr: "localhost:11000",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Cluster: ClusterConfig{JoinAddr: tt.joinAddr},
			}
			if got := cfg.IsBootstrap(); got != tt.want {
				t.Errorf("IsBootstrap() = %v, want %v", got, tt.want)
			}
		})
	}
}
