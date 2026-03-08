package raft

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

const (
	electionTimeoutMin = 150 * time.Millisecond
	electionTimeoutMax = 300 * time.Millisecond
)

func randomElectionTimeout() time.Duration {
	return electionTimeoutMin + time.Duration(rand.Int63n(int64(electionTimeoutMax-electionTimeoutMin)))
}

// runElectionTimer runs the election timeout loop. When the timer fires
// without being reset (by a heartbeat or granting a vote), start an election.
func (rn *RaftNode) runElectionTimer() {
	timeout := randomElectionTimeout()
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case <-rn.stopCh:
			return
		case <-rn.resetTimer:
			timer.Reset(randomElectionTimeout())
		case <-timer.C:
			rn.mu.Lock()
			if rn.Role != Leader {
				rn.startElection()
			}
			rn.mu.Unlock()
			timer.Reset(randomElectionTimeout())
		}
	}
}

// startElection transitions to candidate and requests votes from all peers.
// Must be called with rn.mu held.
func (rn *RaftNode) startElection() {
	rn.CurrentTerm++
	rn.Role = Candidate
	rn.VotedFor = rn.ID
	rn.LeaderID = ""

	term := rn.CurrentTerm
	lastIdx := rn.lastLogIndex()
	lastTerm := rn.lastLogTerm()
	log.Printf("[%s] starting election for term %d", rn.ID, term)

	args := RequestVoteArgs{
		Term:         term,
		CandidateID:  rn.ID,
		LastLogIndex: lastIdx,
		LastLogTerm:  lastTerm,
	}

	votes := 1 // vote for self
	needed := (len(rn.Peers)+1)/2 + 1

	var wg sync.WaitGroup
	var vmu sync.Mutex

	for _, peerID := range rn.Peers {
		wg.Add(1)
		go func(peer string) {
			defer wg.Done()
			reply, err := rn.Transport.SendRequestVote(peer, args)
			if err != nil {
				return
			}

			rn.mu.Lock()
			defer rn.mu.Unlock()

			// If we've moved on, ignore stale replies
			if rn.CurrentTerm != term || rn.Role != Candidate {
				return
			}

			if reply.Term > rn.CurrentTerm {
				rn.stepDown(reply.Term)
				return
			}

			if reply.VoteGranted {
				vmu.Lock()
				votes++
				won := votes >= needed
				vmu.Unlock()
				if won {
					rn.becomeLeader()
				}
			}
		}(peerID)
	}
}

// handleRequestVote processes an incoming RequestVote RPC.
func (rn *RaftNode) HandleRequestVote(args RequestVoteArgs) RequestVoteReply {
	rn.mu.Lock()
	defer rn.mu.Unlock()

	reply := RequestVoteReply{Term: rn.CurrentTerm}

	if args.Term < rn.CurrentTerm {
		return reply
	}

	if args.Term > rn.CurrentTerm {
		rn.stepDown(args.Term)
	}

	reply.Term = rn.CurrentTerm

	// Grant vote if we haven't voted for someone else, and candidate's log is up-to-date
	if (rn.VotedFor == "" || rn.VotedFor == args.CandidateID) && rn.isLogUpToDate(args.LastLogIndex, args.LastLogTerm) {
		rn.VotedFor = args.CandidateID
		reply.VoteGranted = true
		rn.resetElectionTimer()
	}

	return reply
}

// isLogUpToDate returns true if the candidate's log is at least as up-to-date as ours.
func (rn *RaftNode) isLogUpToDate(candidateLastIndex, candidateLastTerm uint64) bool {
	myLastTerm := rn.lastLogTerm()
	myLastIndex := rn.lastLogIndex()

	if candidateLastTerm != myLastTerm {
		return candidateLastTerm > myLastTerm
	}
	return candidateLastIndex >= myLastIndex
}
