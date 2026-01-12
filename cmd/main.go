package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	webserver "github.com/p4trickweiss/distributed-cache/internal/http"
	"github.com/p4trickweiss/distributed-cache/internal/store"
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
)

func init() {
	flag.StringVar(&httpAddr, "haddr", DefaultHTTPAddr, "Set the HTTP bind address")
	flag.StringVar(&raftAddr, "raddr", DefaultRaftAddr, "Set Raft bind address")
	flag.StringVar(&joinAddr, "join", "", "Set join address, if any")
	flag.StringVar(&nodeID, "id", "", "Node ID. If not set, same as Raft bind address")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] <raft-data-path>\n", os.Args[0])
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "No Raft storage directory specified\n")
		os.Exit(1)
	}

	if nodeID == "" {
		nodeID = raftAddr
	}

	raftDir := flag.Arg(0)
	if raftDir == "" {
		log.Fatalln("No Raft storage directory specified")
	}
	if err := os.MkdirAll(raftDir, 0o700); err != nil {
		log.Fatalf("failed to create path for Raft storage: %s", err.Error())
	}

	s := store.New()
	config := store.Config{
		NodeId:    nodeID,
		BindAddr:  raftAddr,
		DataDir:   raftDir,
		Bootstrap: joinAddr == "",
	}
	if err := s.Open(config); err != nil {
		log.Fatalf("failed to open store: %s", err.Error())
	}

	server := webserver.NewServer(httpAddr, s)
	go func() {
		log.Println("Server is starting...")
		if err := server.Start(); err != nil {
			log.Fatal("Server failed to start:", err)
		}
	}()

	if joinAddr != "" {
		if err := join(joinAddr, raftAddr, nodeID); err != nil {
			log.Fatalf("failed to join node at %s: %s", joinAddr, err.Error())
		}
	}

	log.Printf("distributed-cache started successfully, listening on http://%s", httpAddr)

	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	<-terminate
	log.Println("distributed-cache exiting")
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
