package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tristan/distributeddatabase/internal/config"
	"github.com/tristan/distributeddatabase/internal/raft"
	"github.com/tristan/distributeddatabase/internal/server"
	"github.com/tristan/distributeddatabase/internal/transport"
)

func main() {
	cfg := config.Parse()
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Printf("starting node %s (raft=%d, api=%d)", cfg.ID, cfg.RaftPort, cfg.APIPort)

	// Build peer map for transport
	peerAddrs := make(map[string]string)
	var peerIDs []string
	for _, p := range cfg.Peers {
		if p.ID == cfg.ID {
			continue // skip self
		}
		peerAddrs[p.ID] = p.RaftAddr
		peerIDs = append(peerIDs, p.ID)
	}

	tp := transport.NewHTTPTransport(peerAddrs)
	node := raft.NewRaftNode(cfg.ID, peerIDs, tp)

	// Start raft HTTP server for RPCs
	raftServer := server.NewRaftHTTPServer(node)
	raftHTTP := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.RaftPort),
		Handler: raftServer.Handler(),
	}

	go func() {
		log.Printf("[%s] raft server listening on :%d", cfg.ID, cfg.RaftPort)
		if err := raftHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("raft http: %v", err)
		}
	}()

	// Start API server (placeholder for Phase 2, serves status for now)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /raft/status", func(w http.ResponseWriter, r *http.Request) {
		raftServer.Handler().ServeHTTP(w, r)
	})
	apiHTTP := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.APIPort),
		Handler: apiMux,
	}

	go func() {
		log.Printf("[%s] api server listening on :%d", cfg.ID, cfg.APIPort)
		if err := apiHTTP.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("api http: %v", err)
		}
	}()

	// Start raft
	node.Run()

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh
	log.Printf("[%s] shutting down...", cfg.ID)

	node.Stop()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	raftHTTP.Shutdown(ctx)
	apiHTTP.Shutdown(ctx)
}
