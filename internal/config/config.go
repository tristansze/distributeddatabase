package config

import (
	"flag"
	"fmt"
	"strings"
)

type PeerAddr struct {
	ID       string
	RaftAddr string // host:port for raft RPCs
}

type Config struct {
	ID       string
	RaftPort int
	APIPort  int
	Peers    []PeerAddr
	DataDir  string
}

func Parse() Config {
	var (
		id      = flag.String("id", "", "node id (e.g. node1)")
		raft    = flag.Int("raft-port", 9001, "raft RPC port")
		api     = flag.Int("api-port", 8001, "client API port")
		peers   = flag.String("peers", "", "comma-separated peer list: id=host:port,...")
		dataDir = flag.String("data-dir", "data", "data directory for WAL")
	)
	flag.Parse()

	if *id == "" {
		panic("--id is required")
	}

	cfg := Config{
		ID:       *id,
		RaftPort: *raft,
		APIPort:  *api,
		DataDir:  fmt.Sprintf("%s/%s", *dataDir, *id),
	}

	if *peers != "" {
		for _, p := range strings.Split(*peers, ",") {
			parts := strings.SplitN(p, "=", 2)
			if len(parts) != 2 {
				panic(fmt.Sprintf("invalid peer format: %q (expected id=host:port)", p))
			}
			cfg.Peers = append(cfg.Peers, PeerAddr{ID: parts[0], RaftAddr: parts[1]})
		}
	}

	return cfg
}
