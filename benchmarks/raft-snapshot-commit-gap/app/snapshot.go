package main

// Snapshot holds the state captured at a particular point in the Raft log.
type Snapshot struct {
	LastIncludedIndex int
	LastIncludedTerm  int
	Data              map[string]string
}

// InstallSnapshot handles the InstallSnapshot RPC on a follower.
// When a leader determines that a follower is too far behind to catch
// up via AppendEntries, it sends its latest snapshot instead.
//
// Per the Raft paper (Figure 13):
//   1. Save the snapshot data
//   2. Discard any log entries covered by the snapshot
//   3. Reset the state machine from the snapshot
//   4. Update lastApplied to the snapshot's last included index
//
// This handler is called on the receiving (follower) node.
func (n *RaftNode) InstallSnapshot(snap Snapshot) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Ignore stale snapshots
	if snap.LastIncludedIndex <= n.lastApplied {
		return
	}

	n.log.Infof("installing snapshot: lastIncludedIndex=%d lastIncludedTerm=%d",
		snap.LastIncludedIndex, snap.LastIncludedTerm)

	// Save snapshot for crash recovery
	n.snapshot = &snap

	// If the snapshot covers entries beyond our log, reset to just
	// the snapshot boundary. Otherwise keep any log entries after the
	// snapshot point (they may be needed for future consistency checks).
	if snap.LastIncludedIndex > n.raftLog.LastIndex() {
		n.raftLog = NewRaftLog()
		n.raftLog.offset = snap.LastIncludedIndex
		n.raftLog.entries = []LogEntry{{
			Index: snap.LastIncludedIndex,
			Term:  snap.LastIncludedTerm,
		}}
	}
	// Note: when the snapshot covers only part of our log, we keep
	// existing entries intact. The log compaction happens separately
	// when the node takes its own snapshot via TakeSnapshot().

	// Restore state machine from snapshot
	n.stateMachine.Restore(snap.Data)

	// Update lastApplied — the state machine now reflects everything
	// up to and including the snapshot's last index.
	n.lastApplied = snap.LastIncludedIndex
}
