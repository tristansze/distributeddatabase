package raft

// Log operations on the in-memory log.
// Log is 1-indexed: Log[0] is a sentinel entry (index=0, term=0).

func (rn *RaftNode) lastLogIndex() uint64 {
	return uint64(len(rn.Log) - 1)
}

func (rn *RaftNode) lastLogTerm() uint64 {
	return rn.Log[len(rn.Log)-1].Term
}

func (rn *RaftNode) getEntry(index uint64) *LogEntry {
	if index >= uint64(len(rn.Log)) {
		return nil
	}
	return &rn.Log[index]
}

func (rn *RaftNode) appendEntry(entry LogEntry) {
	rn.Log = append(rn.Log, entry)
}

func (rn *RaftNode) truncateFrom(index uint64) {
	if index < uint64(len(rn.Log)) {
		rn.Log = rn.Log[:index]
	}
}
