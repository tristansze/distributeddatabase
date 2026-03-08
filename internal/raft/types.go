package raft

import "sync"


// This file defines everything for passing
// messages between nodes in the system

// What state a node is currently in
type Role int
const (
	Follower Role = iota
	Candidate
	Leader
)

func (r Role) String() string {
	switch r {
	case Follower:
		return "follower"
	case Candidate:
		return "candidate"
	case Leader:
		return "leader"
	default:
		return "unknown"
	}
}

// Command that should be replicated across nodes
type LogEntry struct {
	Index   uint64 `json:"index"`
	Term    uint64 `json:"term"`
	Command []byte `json:"command"`
}

// RequestVote RPC
type RequestVoteArgs struct {
	Term         uint64 `json:"term"`
	CandidateID  string `json:"candidateId"`
	LastLogIndex uint64 `json:"lastLogIndex"`
	LastLogTerm  uint64 `json:"lastLogTerm"`
}

type RequestVoteReply struct {
	Term        uint64 `json:"term"`
	VoteGranted bool   `json:"voteGranted"`
}

// AppendEntries RPC
type AppendEntriesArgs struct {
	Term         uint64     `json:"term"`
	LeaderID     string     `json:"leaderId"`
	PrevLogIndex uint64     `json:"prevLogIndex"`
	PrevLogTerm  uint64     `json:"prevLogTerm"`
	Entries      []LogEntry `json:"entries"`
	LeaderCommit uint64     `json:"leaderCommit"`
}

type AppendEntriesReply struct {
	Term    uint64 `json:"term"`
	Success bool   `json:"success"`
}

// Transport abstracts network communication between raft nodes.
type Transport interface {
	SendRequestVote(peerID string, args RequestVoteArgs) (RequestVoteReply, error)
	SendAppendEntries(peerID string, args AppendEntriesArgs) (AppendEntriesReply, error)
}

// RaftNode holds all of the states/values for a given node
type RaftNode struct {
	mu sync.Mutex

	// Identity
	ID    string
	Peers []string // peer node IDs

	// Persistent state (on all servers)
	CurrentTerm uint64
	VotedFor    string
	Log         []LogEntry // 1-indexed; Log[0] is a dummy entry

	// Volatile state (on all servers)
	CommitIndex uint64
	LastApplied uint64
	Role        Role
	LeaderID    string

	// Volatile state (on leaders)
	NextIndex  map[string]uint64
	MatchIndex map[string]uint64

	// Channels and hooks
	Transport  Transport
	ApplyCh    chan LogEntry
	stopCh     chan struct{}
	resetTimer chan struct{}
}
