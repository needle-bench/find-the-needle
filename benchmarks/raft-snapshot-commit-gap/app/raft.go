package main

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// NodeState represents the Raft node role.
type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

func (s NodeState) String() string {
	switch s {
	case Follower:
		return "Follower"
	case Candidate:
		return "Candidate"
	case Leader:
		return "Leader"
	default:
		return "Unknown"
	}
}

// Logger provides structured logging for a node.
type Logger struct {
	prefix string
	w      io.Writer
}

func NewLogger(nodeID int, w io.Writer) *Logger {
	return &Logger{
		prefix: fmt.Sprintf("[node-%d] ", nodeID),
		w:      w,
	}
}

func (l *Logger) Infof(format string, args ...interface{}) {
	fmt.Fprintf(l.w, l.prefix+format+"\n", args...)
}

// RaftNode is a simplified Raft consensus node.
type RaftNode struct {
	mu sync.Mutex

	id          int
	state       NodeState
	currentTerm int
	votedFor    int
	leaderId    int

	raftLog      *RaftLog
	commitIndex  int
	lastApplied  int
	stateMachine *StateMachine
	snapshot     *Snapshot

	// Leader-only state
	nextIndex  map[int]int
	matchIndex map[int]int

	peers []int
	log   *Logger
}

func NewRaftNode(id int, peers []int) *RaftNode {
	return &RaftNode{
		id:           id,
		state:        Follower,
		currentTerm:  0,
		votedFor:     -1,
		leaderId:     -1,
		raftLog:      NewRaftLog(),
		commitIndex:  0,
		lastApplied:  0,
		stateMachine: NewStateMachine(),
		nextIndex:    make(map[int]int),
		matchIndex:   make(map[int]int),
		peers:        peers,
		log:          NewLogger(id, os.Stdout),
	}
}

// SetLogWriter redirects log output (used in tests to suppress noise).
func (n *RaftNode) SetLogWriter(w io.Writer) {
	n.log = NewLogger(n.id, w)
}

// BecomeLeader transitions this node to leader state.
func (n *RaftNode) BecomeLeader(term int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.state = Leader
	n.currentTerm = term
	n.leaderId = n.id
	n.votedFor = n.id

	// Initialize leader volatile state
	lastIdx := n.raftLog.LastIndex()
	for _, peer := range n.peers {
		n.nextIndex[peer] = lastIdx + 1
		n.matchIndex[peer] = 0
	}

	n.log.Infof("became leader for term %d (lastIndex=%d)", term, lastIdx)
}

// BecomeFollower transitions this node to follower state.
func (n *RaftNode) BecomeFollower(term int, leaderID int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.state = Follower
	n.currentTerm = term
	n.leaderId = leaderID
	n.votedFor = -1

	n.log.Infof("became follower for term %d (leader=%d)", term, leaderID)
}

// ProposeEntry adds a client command to the leader's log. Returns the log index.
func (n *RaftNode) ProposeEntry(command string) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.state != Leader {
		return 0, fmt.Errorf("node %d is not leader", n.id)
	}

	idx := n.raftLog.Append(n.currentTerm, command)
	n.log.Infof("proposed entry index=%d term=%d cmd=%q", idx, n.currentTerm, command)
	return idx, nil
}

// AppendEntries handles the AppendEntries RPC on a follower.
// Returns (term, success).
func (n *RaftNode) AppendEntries(leaderTerm int, leaderID int, prevLogIndex int, prevLogTerm int, entries []LogEntry, leaderCommit int) (int, bool) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// Reply false if term < currentTerm
	if leaderTerm < n.currentTerm {
		return n.currentTerm, false
	}

	// Update term and revert to follower if needed
	if leaderTerm > n.currentTerm {
		n.currentTerm = leaderTerm
		n.state = Follower
		n.votedFor = -1
	}
	n.leaderId = leaderID

	// Consistency check: verify we have the entry at prevLogIndex with prevLogTerm
	if prevLogIndex > 0 {
		prevEntry, err := n.raftLog.Get(prevLogIndex)
		if err != nil {
			n.log.Infof("AppendEntries rejected: missing entry at prevLogIndex=%d", prevLogIndex)
			return n.currentTerm, false
		}
		if prevEntry.Term != prevLogTerm {
			n.log.Infof("AppendEntries rejected: term mismatch at index=%d (have=%d, want=%d)",
				prevLogIndex, prevEntry.Term, prevLogTerm)
			return n.currentTerm, false
		}
	}

	// Append/replace entries
	if len(entries) > 0 {
		// Check for conflicting entries
		for i, entry := range entries {
			existing, err := n.raftLog.Get(entry.Index)
			if err != nil {
				// No entry at this index, append remaining
				for _, e := range entries[i:] {
					n.raftLog.Append(e.Term, e.Command)
				}
				break
			}
			if existing.Term != entry.Term {
				// Conflict — truncate and append remaining
				n.raftLog.ReplaceFrom(entry.Index, entries[i:])
				break
			}
		}
	}

	// Update commitIndex
	if leaderCommit > n.commitIndex {
		lastNewIndex := n.raftLog.LastIndex()
		if leaderCommit < lastNewIndex {
			n.commitIndex = leaderCommit
		} else {
			n.commitIndex = lastNewIndex
		}
	}

	// Apply committed entries to state machine
	n.applyEntries()

	return n.currentTerm, true
}

// applyEntries applies all committed but unapplied entries to the state machine.
// Must be called with n.mu held.
func (n *RaftNode) applyEntries() {
	for n.lastApplied < n.commitIndex {
		n.lastApplied++
		entry, err := n.raftLog.Get(n.lastApplied)
		if err != nil {
			n.log.Infof("ERROR: cannot apply entry %d: %v", n.lastApplied, err)
			break
		}
		if entry.Command == "" {
			continue // skip sentinel/no-op entries
		}
		if err := n.stateMachine.Apply(entry.Command); err != nil {
			n.log.Infof("ERROR: state machine apply failed for entry %d: %v", n.lastApplied, err)
		} else {
			n.log.Infof("applied entry %d: %s", n.lastApplied, entry.Command)
		}
	}
}

// CommitEntries sets the commit index (simulating quorum acknowledgement)
// and applies newly committed entries. Used by the leader.
func (n *RaftNode) CommitEntries(upToIndex int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if upToIndex > n.commitIndex {
		n.commitIndex = upToIndex
		n.log.Infof("commitIndex advanced to %d", n.commitIndex)
		n.applyEntries()
	}
}

// TakeSnapshot captures the current state machine state as a snapshot.
func (n *RaftNode) TakeSnapshot() Snapshot {
	n.mu.Lock()
	defer n.mu.Unlock()

	data := n.stateMachine.Snapshot()
	snap := Snapshot{
		LastIncludedIndex: n.lastApplied,
		LastIncludedTerm:  n.currentTerm,
		Data:              data,
	}
	n.snapshot = &snap

	// Compact the log up to the snapshot point
	n.raftLog.CompactUpTo(snap.LastIncludedIndex)

	n.log.Infof("snapshot taken: lastIncludedIndex=%d, lastIncludedTerm=%d, keys=%d",
		snap.LastIncludedIndex, snap.LastIncludedTerm, len(data))

	return snap
}

// PrepareAppendEntries builds AppendEntries arguments for a specific peer.
// Returns (prevLogIndex, prevLogTerm, entries, commitIndex).
func (n *RaftNode) PrepareAppendEntries(peerID int) (int, int, []LogEntry, int) {
	n.mu.Lock()
	defer n.mu.Unlock()

	nextIdx := n.nextIndex[peerID]
	prevIdx := nextIdx - 1

	var prevTerm int
	if prevIdx > 0 {
		if entry, err := n.raftLog.Get(prevIdx); err == nil {
			prevTerm = entry.Term
		}
	}

	entries := n.raftLog.Slice(nextIdx, n.raftLog.LastIndex())

	return prevIdx, prevTerm, entries, n.commitIndex
}

// UpdateNextIndex updates the leader's tracking of a peer's log state
// after a successful AppendEntries.
func (n *RaftNode) UpdateNextIndex(peerID int, matchIndex int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.matchIndex[peerID] = matchIndex
	n.nextIndex[peerID] = matchIndex + 1
}

// GetState returns a snapshot of the node's current state for inspection.
func (n *RaftNode) GetState() (commitIndex, lastApplied, lastLogIndex int) {
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.commitIndex, n.lastApplied, n.raftLog.LastIndex()
}

// Restart simulates a node restart. Volatile state (commitIndex, lastApplied)
// is lost per the Raft paper. Persistent state (log, currentTerm, votedFor,
// snapshot) is retained.
//
// Recovery proceeds in two phases:
//   1. If a snapshot exists, restore the state machine from it.
//   2. Replay any log entries between the snapshot boundary and commitIndex
//      to recover state that was committed after the last snapshot.
//
// The node's lastApplied is derived from commitIndex after recovery, since
// all entries up to commitIndex should be applied to the state machine.
func (n *RaftNode) Restart() {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.log.Infof("restarting node (commitIndex=%d, lastApplied=%d)", n.commitIndex, n.lastApplied)

	// Persistent state survives: currentTerm, votedFor, raftLog, snapshot
	savedCommitIndex := n.commitIndex

	// Volatile state is lost on restart
	n.state = Follower
	n.leaderId = -1
	n.commitIndex = 0
	n.lastApplied = 0
	n.stateMachine = NewStateMachine()

	// Recovery phase 1: Restore from snapshot if available
	snapshotAppliedTo := 0
	if n.snapshot != nil {
		n.stateMachine.Restore(n.snapshot.Data)
		snapshotAppliedTo = n.snapshot.LastIncludedIndex
		n.log.Infof("restored state machine from snapshot (lastIncludedIndex=%d)", snapshotAppliedTo)
	}

	// Recovery phase 2: Replay committed log entries after the snapshot.
	// Set lastApplied to the snapshot boundary, then advance commitIndex
	// to the persisted value so applyEntries() replays the gap.
	n.lastApplied = snapshotAppliedTo
	if savedCommitIndex > n.lastApplied {
		n.commitIndex = savedCommitIndex
		n.log.Infof("replaying log entries %d..%d", n.lastApplied+1, savedCommitIndex)
		n.applyEntries()
	} else {
		// commitIndex <= snapshot boundary. Set lastApplied from commit index
		// to maintain the invariant lastApplied <= commitIndex.
		// If commitIndex was properly set during InstallSnapshot, this path
		// would correctly set lastApplied = snapshotAppliedTo. But if commitIndex
		// was never updated, savedCommitIndex could be 0, and lastApplied gets
		// dragged back down to match.
		n.lastApplied = savedCommitIndex
		n.commitIndex = savedCommitIndex
	}

	n.log.Infof("recovery complete: commitIndex=%d lastApplied=%d", n.commitIndex, n.lastApplied)
}
