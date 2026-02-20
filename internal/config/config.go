package config

import (
	"fmt"
	"os"
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
	ID      string
	DataDir string `mapstructure:"data_dir"`
}

type HTTPConfig struct {
	BindAddr string `mapstructure:"bind_addr"`
}

type RaftConfig struct {
	BindAddr string `mapstructure:"bind_addr"`
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
	v.SetDefault("node.data_dir", "./data")
	v.SetDefault("http.bind_addr", "localhost:11000")
	v.SetDefault("raft.bind_addr", "localhost:12000")
	v.SetDefault("cluster.join_addr", "")
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.json", false)
}

func (c *Config) Validate() error {
	if c.Node.DataDir == "" {
		return fmt.Errorf("node.data_dir is required")
	}

	if c.HTTP.BindAddr == "" {
		return fmt.Errorf("http.bind_addr is required")
	}

	if c.Raft.BindAddr == "" {
		return fmt.Errorf("raft.bind_addr is required")
	}

	// Validate log level
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
