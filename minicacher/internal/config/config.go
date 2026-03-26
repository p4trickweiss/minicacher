package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Node    NodeConfig
	HTTP    HTTPConfig
	Raft    RaftConfig
	Cluster ClusterConfig
	Logging LoggingConfig
}

type NodeConfig struct {
	ID       string
	BindAddr string `mapstructure:"bind_addr"`
	DataDir  string `mapstructure:"data_dir"`
}

type HTTPConfig struct {
	Port int
}

type RaftConfig struct {
	Port int
}

// HTTPAddr returns the full HTTP bind address (host:port).
func (c *Config) HTTPAddr() string {
	return net.JoinHostPort(c.Node.BindAddr, strconv.Itoa(c.HTTP.Port))
}

// RaftAddr returns the full Raft bind address (host:port).
func (c *Config) RaftAddr() string {
	return net.JoinHostPort(c.Node.BindAddr, strconv.Itoa(c.Raft.Port))
}

type ClusterConfig struct {
	JoinAddr string `mapstructure:"join_addr"`
}

type LoggingConfig struct {
	Level string
	JSON  bool
}

// Load loads configuration from file and environment variables
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Set config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
	}

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// Check if it's a "file not found" error
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found, use defaults + env vars
		} else if os.IsNotExist(err) {
			// Explicit config path doesn't exist, use defaults + env vars
		} else {
			// Other error (e.g., parse error)
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Environment variables
	v.SetEnvPrefix("DCACHE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("node.id", "")
	v.SetDefault("node.bind_addr", "localhost")
	v.SetDefault("node.data_dir", "./data")
	v.SetDefault("http.port", 11000)
	v.SetDefault("raft.port", 12000)
	v.SetDefault("cluster.join_addr", "")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.json", false)
}

func (c *Config) Validate() error {
	if c.Node.BindAddr == "" {
		return fmt.Errorf("node.bind_addr is required")
	}

	if c.Node.DataDir == "" {
		return fmt.Errorf("node.data_dir is required")
	}

	if c.HTTP.Port <= 0 {
		return fmt.Errorf("http.port must be a positive integer")
	}

	if c.Raft.Port <= 0 {
		return fmt.Errorf("raft.port must be a positive integer")
	}

	validLevels := map[string]bool{
		"debug": true,
		"info":  true,
		"warn":  true,
		"error": true,
	}
	if !validLevels[c.Logging.Level] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, warn, or error)", c.Logging.Level)
	}

	return nil
}

// IsBootstrap returns true if this node should bootstrap a new cluster
func (c *Config) IsBootstrap() bool {
	return c.Cluster.JoinAddr == ""
}
