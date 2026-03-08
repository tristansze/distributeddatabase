package server

import (
	"encoding/json"
	"net/http"

	"github.com/tristan/distributeddatabase/internal/raft"
)

// RaftHTTPServer handles incoming Raft RPCs over HTTP.
type RaftHTTPServer struct {
	node *raft.RaftNode
	mux  *http.ServeMux
}

func NewRaftHTTPServer(node *raft.RaftNode) *RaftHTTPServer {
	s := &RaftHTTPServer{node: node, mux: http.NewServeMux()}
	s.mux.HandleFunc("POST /raft/request-vote", s.handleRequestVote)
	s.mux.HandleFunc("POST /raft/append-entries", s.handleAppendEntries)
	s.mux.HandleFunc("GET /raft/status", s.handleStatus)
	return s
}

func (s *RaftHTTPServer) Handler() http.Handler {
	return s.mux
}

func (s *RaftHTTPServer) handleRequestVote(w http.ResponseWriter, r *http.Request) {
	var args raft.RequestVoteArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	reply := s.node.HandleRequestVote(args)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reply)
}

func (s *RaftHTTPServer) handleAppendEntries(w http.ResponseWriter, r *http.Request) {
	var args raft.AppendEntriesArgs
	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	reply := s.node.HandleAppendEntries(args)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(reply)
}

func (s *RaftHTTPServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.node.Status())
}
