package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/p4trickweiss/distributed-cache/internal/config"
	webserver "github.com/p4trickweiss/distributed-cache/internal/http"
	"github.com/p4trickweiss/distributed-cache/internal/node"
)

var configPath string

func init() {
	flag.StringVar(&configPath, "config", "", "Path to config file (optional, uses defaults if not specified)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Setup structured logging
	setupLogging(cfg.Logging)

	// Set node ID default if not specified
	nodeID := cfg.Node.ID
	if nodeID == "" {
		nodeID = cfg.Raft.BindAddr
	}

	slog.Info("starting distributed-cache",
		"node_id", nodeID,
		"http_addr", cfg.HTTP.BindAddr,
		"raft_addr", cfg.Raft.BindAddr,
		"join_addr", cfg.Cluster.JoinAddr)

	// Create data directory
	if err := os.MkdirAll(cfg.Node.DataDir, 0o700); err != nil {
		log.Fatalf("failed to create data directory: %v", err)
	}

	// Initialize node
	n := node.New()
	nodeConfig := node.Config{
		NodeId:    nodeID,
		BindAddr:  cfg.Raft.BindAddr,
		DataDir:   cfg.Node.DataDir,
		Bootstrap: cfg.IsBootstrap(),
	}
	if err := n.Open(nodeConfig); err != nil {
		log.Fatalf("failed to open node: %v", err)
	}

	// Start HTTP server
	server := webserver.NewServer(cfg.HTTP.BindAddr, n, nodeID)
	go func() {
		slog.Info("server is starting")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	// Join cluster if needed
	if cfg.Cluster.JoinAddr != "" {
		if err := join(cfg.Cluster.JoinAddr, cfg.Raft.BindAddr, nodeID); err != nil {
			log.Fatalf("failed to join cluster: %v", err)
		}
	}

	slog.Info("distributed-cache started successfully",
		"http_addr", cfg.HTTP.BindAddr,
		"is_leader", n.IsLeader())

	// Wait for termination signal
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	<-terminate

	slog.Info("shutdown signal received, starting graceful shutdown")

	// Graceful shutdown with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Shutdown HTTP server first (stop accepting new requests)
	slog.Info("shutting down HTTP server")
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown failed",
			"error", err)
	} else {
		slog.Info("HTTP server shutdown complete")
	}

	// Close Raft and node
	slog.Info("shutting down node")
	if err := n.Close(); err != nil {
		slog.Error("node shutdown failed",
			"error", err)
		os.Exit(1)
	}

	slog.Info("distributed-cache shutdown complete")
}

func setupLogging(cfg config.LoggingConfig) {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.JSON {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
}

func join(joinAddr, raftAddr, nodeID string) error {
	b, err := json.Marshal(map[string]string{"addr": raftAddr, "id": nodeID})
	if err != nil {
		return err
	}
	resp, err := http.Post(fmt.Sprintf("http://%s/join", joinAddr), "application/json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}
