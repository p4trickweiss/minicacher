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

	webserver "github.com/p4trickweiss/distributed-cache/internal/http"
	"github.com/p4trickweiss/distributed-cache/internal/node"
)

const (
	DefaultHTTPAddr = "localhost:11000"
	DefaultRaftAddr = "localhost:12000"
)

var (
	httpAddr string
	raftAddr string
	joinAddr string
	nodeID   string
	logLevel string
	logJSON  bool
)

func init() {
	flag.StringVar(&httpAddr, "haddr", DefaultHTTPAddr, "Set the HTTP bind address")
	flag.StringVar(&raftAddr, "raddr", DefaultRaftAddr, "Set Raft bind address")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&nodeID, "id", "", "Node ID. If not set, same as Raft bind address")
	flag.StringVar(&logLevel, "log-level", "info", "Log level (debug, info, warn, error)")
	flag.BoolVar(&logJSON, "log-json", false, "Output logs in JSON format")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <raft-data-path>\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Setup structured logging
	setupLogging()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "No Raft storage directory specified\n")
		os.Exit(1)
	}

	if nodeID == "" {
		nodeID = raftAddr
	}

	slog.Info("starting distributed-cache",
		"node_id", nodeID,
		"http_addr", httpAddr,
		"raft_addr", raftAddr,
		"join_addr", joinAddr)

	raftDir := flag.Arg(0)
	if raftDir == "" {
		log.Fatalln("No Raft storage directory specified")
	}
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		log.Fatalf("failed to create path for Raft storage: %s", err.Error())
	}

	n := node.New()
	config := node.Config{
		NodeId:    nodeID,
		BindAddr:  raftAddr,
		DataDir:   raftDir,
		Bootstrap: joinAddr == "",
	}
	if err := n.Open(config); err != nil {
		log.Fatalf("failed to open node: %s", err.Error())
	}

	server := webserver.NewServer(httpAddr, n, nodeID)
	go func() {
		slog.Info("server is starting")
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed to start",
				"error", err)
			os.Exit(1)
		}
	}()

	if joinAddr != "" {
		if err := join(joinAddr, raftAddr, nodeID); err != nil {
			log.Fatalf("failed to join node at %s: %s", joinAddr, err.Error())
		}
	}

	slog.Info("distributed-cache started successfully",
		"http_addr", httpAddr,
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

func setupLogging() {
	// Parse log level
	var level slog.Level
	switch logLevel {
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

	// Create handler based on format
	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if logJSON {
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
