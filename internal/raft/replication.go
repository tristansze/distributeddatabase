package raft

import (
	"log"
	"time"
)

const heartbeatInterval = 50 * time.Millisecond

// becomeLeader transitions to leader and initializes leader state.
// Must be called with rn.mu held.
func (rn *RaftNode) becomeLeader() {
	if rn.Role == Leader {
		return
	}
	log.Printf("[%s] became leader for term %d", rn.ID, rn.CurrentTerm)
	rn.Role = Leader
	rn.LeaderID = rn.ID

	// Initialize nextIndex and matchIndex
	rn.NextIndex = make(map[string]uint64)
	rn.MatchIndex = make(map[string]uint64)
	lastIdx := rn.lastLogIndex()
	for _, peer := range rn.Peers {
		rn.NextIndex[peer] = lastIdx + 1
		rn.MatchIndex[peer] = 0
	}

	// Send immediate heartbeat
	go rn.sendHeartbeats()
}

// runHeartbeatLoop sends periodic heartbeats while this node is leader.
func (rn *RaftNode) runHeartbeatLoop() {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rn.stopCh:
			return
		case <-ticker.C:
			rn.mu.Lock()
			isLeader := rn.Role == Leader
			rn.mu.Unlock()
			if isLeader {
				rn.sendHeartbeats()
			}
		}
	}
}

// sendHeartbeats sends AppendEntries RPCs (empty for heartbeats) to all peers.
func (rn *RaftNode) sendHeartbeats() {
	rn.mu.Lock()
	if rn.Role != Leader {
		rn.mu.Unlock()
		return
	}
	term := rn.CurrentTerm
	leaderID := rn.ID
	commitIndex := rn.CommitIndex
	peers := make([]string, len(rn.Peers))
	copy(peers, rn.Peers)

	// Build per-peer args
	type peerArgs struct {
		id   string
		args AppendEntriesArgs
	}
	var sends []peerArgs
	for _, peer := range peers {
		nextIdx := rn.NextIndex[peer]
		prevIdx := nextIdx - 1
		prevTerm := uint64(0)
		if entry := rn.getEntry(prevIdx); entry != nil {
			prevTerm = entry.Term
		}

		// Collect entries to send
		var entries []LogEntry
		lastIdx := rn.lastLogIndex()
		if nextIdx <= lastIdx {
			entries = make([]LogEntry, lastIdx-nextIdx+1)
			copy(entries, rn.Log[nextIdx:lastIdx+1])
		}

		sends = append(sends, peerArgs{
			id: peer,
			args: AppendEntriesArgs{
				Term:         term,
				LeaderID:     leaderID,
				PrevLogIndex: prevIdx,
				PrevLogTerm:  prevTerm,
				Entries:      entries,
				LeaderCommit: commitIndex,
			},
		})
	}
	rn.mu.Unlock()

	for _, s := range sends {
		go func(peer string, args AppendEntriesArgs) {
			reply, err := rn.Transport.SendAppendEntries(peer, args)
			if err != nil {
				return
			}
			rn.handleAppendEntriesReply(peer, args, reply)
		}(s.id, s.args)
	}
}

// handleAppendEntriesReply processes a response from a peer.
func (rn *RaftNode) handleAppendEntriesReply(peerID string, args AppendEntriesArgs, reply AppendEntriesReply) {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	if reply.Term > rn.CurrentTerm {
		rn.stepDown(reply.Term)
		return
	}

	if rn.Role != Leader || rn.CurrentTerm != args.Term {
		return
	}

	if reply.Success {
		// Update nextIndex and matchIndex
		newMatchIdx := args.PrevLogIndex + uint64(len(args.Entries))
		if newMatchIdx > rn.MatchIndex[peerID] {
			rn.MatchIndex[peerID] = newMatchIdx
		}
		rn.NextIndex[peerID] = newMatchIdx + 1
		rn.advanceCommitIndex()
	} else {
		// Decrement nextIndex and retry
		if rn.NextIndex[peerID] > 1 {
			rn.NextIndex[peerID]--
		}
	}
}

// HandleAppendEntries processes an incoming AppendEntries RPC.
func (rn *RaftNode) HandleAppendEntries(args AppendEntriesArgs) AppendEntriesReply {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	reply := AppendEntriesReply{Term: rn.CurrentTerm}

	if args.Term < rn.CurrentTerm {
		return reply
	}

	if args.Term > rn.CurrentTerm {
		rn.stepDown(args.Term)
	}

	// Valid leader heartbeat — reset election timer and record leader
	rn.Role = Follower
	rn.LeaderID = args.LeaderID
	rn.resetElectionTimer()
	reply.Term = rn.CurrentTerm

	// Log consistency check
	if args.PrevLogIndex > 0 {
		if args.PrevLogIndex >= uint64(len(rn.Log)) {
			return reply // we don't have PrevLogIndex
		}
		if rn.Log[args.PrevLogIndex].Term != args.PrevLogTerm {
			// Conflict: delete existing entry and all that follow
			rn.truncateFrom(args.PrevLogIndex)
			return reply
		}
	}

	// Append new entries (skip entries we already have)
	for i, entry := range args.Entries {
		idx := args.PrevLogIndex + 1 + uint64(i)
		if idx < uint64(len(rn.Log)) {
			if rn.Log[idx].Term != entry.Term {
				rn.truncateFrom(idx)
				rn.appendEntry(entry)
			}
			// else: already have this entry, skip
		} else {
			rn.appendEntry(entry)
		}
	}

	// Update commit index
	if args.LeaderCommit > rn.CommitIndex {
		lastNewIdx := args.PrevLogIndex + uint64(len(args.Entries))
		if args.LeaderCommit < lastNewIdx {
			rn.CommitIndex = args.LeaderCommit
		} else {
			rn.CommitIndex = lastNewIdx
		}
		rn.applyCommitted()
	}

	reply.Success = true
	return reply
}

// advanceCommitIndex checks if there's an N such that a majority of matchIndex[i] >= N
// and Log[N].term == currentTerm, then sets commitIndex = N.
// Must be called with rn.mu held.
func (rn *RaftNode) advanceCommitIndex() {
	for n := rn.CommitIndex + 1; n <= rn.lastLogIndex(); n++ {
		if rn.Log[n].Term != rn.CurrentTerm {
			continue
		}
		matches := 1 // leader counts
		for _, peer := range rn.Peers {
			if rn.MatchIndex[peer] >= n {
				matches++
			}
		}
		if matches >= (len(rn.Peers)+1)/2+1 {
			rn.CommitIndex = n
		}
	}
	rn.applyCommitted()
}

// applyCommitted sends committed but unapplied entries to the apply channel.
// Must be called with rn.mu held.
func (rn *RaftNode) applyCommitted() {
	for rn.LastApplied < rn.CommitIndex {
		rn.LastApplied++
		entry := rn.Log[rn.LastApplied]
		// Non-blocking send; Phase 2 will add the consumer
		select {
		case rn.ApplyCh <- entry:
		default:
		}
	}
}
