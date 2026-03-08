package raft

import "log"

// NewRaftNode creates a new Raft node.
func NewRaftNode(id string, peers []string, transport Transport) *RaftNode {
	rn := &RaftNode{
		ID:          id,
		Peers:       peers,
		CurrentTerm: 0,
		VotedFor:    "",
		Log:         []LogEntry{{Index: 0, Term: 0}}, // sentinel
		CommitIndex: 0,
		LastApplied: 0,
		Role:        Follower,
		Transport:   transport,
		ApplyCh:     make(chan LogEntry, 256),
		stopCh:      make(chan struct{}),
		resetTimer:  make(chan struct{}, 1),
	}
	return rn
}

// Run starts the Raft node's background goroutines.
func (rn *RaftNode) Run() {
	log.Printf("[%s] starting raft node", rn.ID)
	go rn.runElectionTimer()
	go rn.runHeartbeatLoop()
}

// Stop shuts down the Raft node.
func (rn *RaftNode) Stop() {
	close(rn.stopCh)
}

// stepDown reverts to follower with the given term.
// Must be called with rn.mu held.
func (rn *RaftNode) stepDown(term uint64) {
	rn.CurrentTerm = term
	rn.Role = Follower
	rn.VotedFor = ""
	rn.LeaderID = ""
}

// resetElectionTimer signals the election timer to reset.
func (rn *RaftNode) resetElectionTimer() {
	select {
	case rn.resetTimer <- struct{}{}:
	default:
	}
}

// Status returns a snapshot of the node's current state (for debug endpoint).
type Status struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	Term        uint64 `json:"term"`
	LeaderID    string `json:"leaderId"`
	CommitIndex uint64 `json:"commitIndex"`
	LastApplied uint64 `json:"lastApplied"`
	LogLength   int    `json:"logLength"`
}

func (rn *RaftNode) Status() Status {
	rn.mu.Lock()
	defer rn.mu.Unlock()
	return Status{
		ID:          rn.ID,
		Role:        rn.Role.String(),
		Term:        rn.CurrentTerm,
		LeaderID:    rn.LeaderID,
		CommitIndex: rn.CommitIndex,
		LastApplied: rn.LastApplied,
		LogLength:   len(rn.Log),
	}
}
