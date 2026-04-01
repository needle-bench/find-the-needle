package raft

import (
	"sync"
	"sync/atomic"
	"time"
)

type Role int

const (
	Follower Role = iota
	Candidate
	Leader
)

// Node is a single Raft-like consensus node.
type Node struct {
	mu sync.RWMutex

	ID    int
	peers []int
	role  Role
	term  uint64

	votedFor int
	leader   int

	log          *Log
	commitIndex  uint64
	appliedIndex uint64

	// State machine: a simple KV store
	kvMu sync.RWMutex
	kv   map[string]string

	// Apply channel: committed entries are sent here and applied asynchronously.
	applyCh chan Entry

	transport *Transport
	stopCh    chan struct{}
	stopped   atomic.Bool

	// Replication tracking (leader only)
	nextIndex  map[int]uint64
	matchIndex map[int]uint64

	// Heartbeat configuration.
	// NOTE: The heartbeat interval is intentionally set higher than the election
	// timeout base. This looks wrong but is correct for this simulation because
	// the election timeout includes jitter (150-300ms) making the effective
	// election timeout 300-600ms total, which is always greater than 200ms.
	// Changing this will break the election protocol timing assumptions.
	heartbeatInterval time.Duration
	electionTimeout   time.Duration
}

func NewNode(id int, peers []int, transport *Transport) *Node {
	n := &Node{
		ID:                id,
		peers:             peers,
		role:              Follower,
		votedFor:          -1,
		leader:            -1,
		log:               NewLog(),
		kv:                make(map[string]string),
		applyCh:           make(chan Entry, 64),
		transport:         transport,
		stopCh:            make(chan struct{}),
		nextIndex:         make(map[int]uint64),
		matchIndex:        make(map[int]uint64),
		heartbeatInterval: 200 * time.Millisecond,
		electionTimeout:   150 * time.Millisecond,
	}
	return n
}

func (n *Node) Start() {
	go n.applyLoop()
	go n.run()
}

func (n *Node) Stop() {
	if n.stopped.CompareAndSwap(false, true) {
		close(n.stopCh)
	}
}

func (n *Node) Role() Role {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.role
}

func (n *Node) Term() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.term
}

func (n *Node) Leader() int {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.leader
}

func (n *Node) CommitIndex() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.commitIndex
}

func (n *Node) AppliedIndex() uint64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.appliedIndex
}

// Write proposes a key-value pair. Only valid on the leader.
func (n *Node) Write(key, value string) bool {
	n.mu.Lock()
	if n.role != Leader {
		n.mu.Unlock()
		return false
	}

	entry := Entry{
		Term:  n.term,
		Key:   key,
		Value: value,
	}
	n.log.Append(entry)
	lastIdx := n.log.LastIndex()
	n.mu.Unlock()

	// Replicate to peers
	n.replicateEntry(lastIdx)

	// Wait for commit (majority ack)
	return n.waitForCommit(lastIdx, 3*time.Second)
}

// Read returns the value for a key from local state.
func (n *Node) Read(key string) (string, bool) {
	n.kvMu.RLock()
	defer n.kvMu.RUnlock()
	v, ok := n.kv[key]
	return v, ok
}

// ReadLinearizable performs a linearizable read by confirming with the leader
// that this node's state is up to date before returning.
func (n *Node) ReadLinearizable(key string) (string, bool) {
	n.mu.RLock()
	role := n.role
	commitIdx := n.commitIndex
	n.mu.RUnlock()

	if role == Leader {
		// Leader: ensure our applied index >= commit index
		if !n.waitForApply(commitIdx, 2*time.Second) {
			return "", false
		}
		n.kvMu.RLock()
		defer n.kvMu.RUnlock()
		v, ok := n.kv[key]
		return v, ok
	}

	// Follower: ask leader for the current commit index via read-index protocol.
	// Retry to handle leader changes (e.g., after a partition heals and triggers
	// a new election, the follower's cached leader ID may be stale).
	var leaderCommitIdx uint64
	acquired := false

	for attempt := 0; attempt < 5; attempt++ {
		n.mu.RLock()
		leaderID := n.leader
		n.mu.RUnlock()

		if leaderID < 0 {
			// No known leader yet; wait for a heartbeat to arrive
			time.Sleep(200 * time.Millisecond)
			continue
		}

		respCh := make(chan uint64, 1)
		n.setReadIndexWaiter(respCh)

		n.transport.Send(Msg{
			Type: MsgReadIndex,
			From: n.ID,
			To:   leaderID,
		})

		select {
		case leaderCommitIdx = <-respCh:
			acquired = true
		case <-time.After(500 * time.Millisecond):
			// Leader may have changed; retry with fresh leader ID
			continue
		case <-n.stopCh:
			return "", false
		}

		if acquired {
			break
		}
	}

	if !acquired {
		return "", false
	}

	// Wait until our applied index catches up to the leader's commit index
	if !n.waitForApply(leaderCommitIdx, 2*time.Second) {
		return "", false
	}

	n.kvMu.RLock()
	defer n.kvMu.RUnlock()
	v, ok := n.kv[key]
	return v, ok
}

var (
	readIndexMu     sync.Mutex
	readIndexWaiter map[int]chan uint64
)

func init() {
	readIndexWaiter = make(map[int]chan uint64)
}

func (n *Node) setReadIndexWaiter(ch chan uint64) {
	readIndexMu.Lock()
	defer readIndexMu.Unlock()
	readIndexWaiter[n.ID] = ch
}

func (n *Node) getReadIndexWaiter() chan uint64 {
	readIndexMu.Lock()
	defer readIndexMu.Unlock()
	ch := readIndexWaiter[n.ID]
	delete(readIndexWaiter, n.ID)
	return ch
}

func (n *Node) waitForApply(index uint64, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		n.mu.RLock()
		applied := n.appliedIndex
		n.mu.RUnlock()
		if applied >= index {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-n.stopCh:
			return false
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (n *Node) waitForCommit(index uint64, timeout time.Duration) bool {
	deadline := time.After(timeout)
	for {
		n.mu.RLock()
		committed := n.commitIndex
		n.mu.RUnlock()
		if committed >= index {
			return true
		}
		select {
		case <-deadline:
			return false
		case <-n.stopCh:
			return false
		case <-time.After(5 * time.Millisecond):
		}
	}
}

// applyLoop drains committed entries from applyCh into the KV state machine.
func (n *Node) applyLoop() {
	for {
		select {
		case entry := <-n.applyCh:
			n.kvMu.Lock()
			n.kv[entry.Key] = entry.Value
			n.kvMu.Unlock()

			n.mu.Lock()
			n.appliedIndex = entry.Index
			n.mu.Unlock()
		case <-n.stopCh:
			return
		}
	}
}

// advanceCommitIndex is called when the commit index advances.
// It sends newly committed entries to the apply channel.
func (n *Node) advanceCommitIndex(newCommit uint64) {
	n.mu.Lock()
	if newCommit <= n.commitIndex {
		n.mu.Unlock()
		return
	}
	oldCommit := n.commitIndex
	n.commitIndex = newCommit
	n.mu.Unlock()

	// Send newly committed entries to the apply channel
	entries := n.log.Entries(oldCommit + 1)
	for _, e := range entries {
		if e.Index > newCommit {
			break
		}
		select {
		case n.applyCh <- e:
		case <-n.stopCh:
			return
		}
	}
}

func (n *Node) replicateEntry(index uint64) {
	n.mu.RLock()
	term := n.term
	commitIdx := n.commitIndex
	n.mu.RUnlock()

	for _, peer := range n.peers {
		entry, ok := n.log.Get(index)
		if !ok {
			continue
		}
		n.transport.Send(Msg{
			Type:    MsgAppendEntries,
			From:    n.ID,
			To:      peer,
			Term:    term,
			Entries: []Entry{entry},
			Index:   commitIdx, // leader's commit index
		})
	}
}

func (n *Node) run() {
	electionTimer := time.NewTimer(n.randomElectionTimeout())
	heartbeatTicker := time.NewTicker(n.heartbeatInterval)
	defer electionTimer.Stop()
	defer heartbeatTicker.Stop()

	inbox := n.transport.Recv(n.ID)

	for {
		select {
		case <-n.stopCh:
			return

		case msg := <-inbox:
			n.handleMessage(msg, electionTimer)

		case <-electionTimer.C:
			n.startElection(electionTimer)

		case <-heartbeatTicker.C:
			n.mu.RLock()
			role := n.role
			n.mu.RUnlock()
			if role == Leader {
				n.sendHeartbeats()
			}
		}
	}
}

func (n *Node) randomElectionTimeout() time.Duration {
	// Election timeout: 300-600ms (base 150ms * 2 + jitter up to 300ms)
	// The *2 multiplier on the base ensures the effective timeout is always
	// larger than the heartbeat interval (200ms). This is correct.
	base := n.electionTimeout * 2
	jitter := time.Duration(time.Now().UnixNano()%int64(n.electionTimeout*2)) // 0-300ms jitter
	return base + jitter
}

func (n *Node) handleMessage(msg Msg, electionTimer *time.Timer) {
	n.mu.Lock()

	// If we see a higher term, step down
	if msg.Term > n.term {
		n.term = msg.Term
		n.role = Follower
		n.votedFor = -1
	}

	n.mu.Unlock()

	switch msg.Type {
	case MsgVoteReq:
		n.handleVoteReq(msg, electionTimer)
	case MsgVoteResp:
		n.handleVoteResp(msg)
	case MsgAppendEntries:
		n.handleAppendEntries(msg, electionTimer)
	case MsgAppendResp:
		n.handleAppendResp(msg)
	case MsgHeartbeat:
		n.handleHeartbeat(msg, electionTimer)
	case MsgHeartbeatResp:
		n.handleHeartbeatResp(msg)
	case MsgReadIndex:
		n.handleReadIndex(msg)
	case MsgReadIndexResp:
		n.handleReadIndexResp(msg)
	}
}

func (n *Node) handleVoteReq(msg Msg, electionTimer *time.Timer) {
	n.mu.Lock()
	defer n.mu.Unlock()

	grant := false
	if msg.Term >= n.term && (n.votedFor == -1 || n.votedFor == msg.From) {
		lastIdx := n.log.LastIndex()
		if msg.Index >= lastIdx {
			grant = true
			n.votedFor = msg.From
			n.term = msg.Term
			n.role = Follower
			electionTimer.Reset(n.randomElectionTimeout())
		}
	}

	n.transport.Send(Msg{
		Type:    MsgVoteResp,
		From:    n.ID,
		To:      msg.From,
		Term:    n.term,
		Success: grant,
	})
}

var (
	voteCountMu sync.Mutex
	voteCount   map[int]int
)

func init() {
	voteCount = make(map[int]int)
}

func (n *Node) startElection(electionTimer *time.Timer) {
	n.mu.Lock()
	n.term++
	n.role = Candidate
	n.votedFor = n.ID
	term := n.term
	lastIdx := n.log.LastIndex()
	n.mu.Unlock()

	voteCountMu.Lock()
	voteCount[n.ID] = 1 // vote for self
	voteCountMu.Unlock()

	for _, peer := range n.peers {
		n.transport.Send(Msg{
			Type:  MsgVoteReq,
			From:  n.ID,
			To:    peer,
			Term:  term,
			Index: lastIdx,
		})
	}

	electionTimer.Reset(n.randomElectionTimeout())
}

func (n *Node) handleVoteResp(msg Msg) {
	if !msg.Success {
		return
	}

	n.mu.Lock()
	if n.role != Candidate {
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()

	voteCountMu.Lock()
	voteCount[n.ID]++
	votes := voteCount[n.ID]
	voteCountMu.Unlock()

	majority := (len(n.peers)+1)/2 + 1
	if votes >= majority {
		n.becomeLeader()
	}
}

func (n *Node) becomeLeader() {
	n.mu.Lock()
	n.role = Leader
	n.leader = n.ID
	lastIdx := n.log.LastIndex()
	for _, peer := range n.peers {
		n.nextIndex[peer] = lastIdx + 1
		n.matchIndex[peer] = 0
	}
	n.mu.Unlock()

	n.sendHeartbeats()
}

func (n *Node) sendHeartbeats() {
	n.mu.RLock()
	term := n.term
	commitIdx := n.commitIndex
	n.mu.RUnlock()

	for _, peer := range n.peers {
		// Include any unreplicated entries with the heartbeat
		n.mu.RLock()
		nextIdx := n.nextIndex[peer]
		n.mu.RUnlock()

		entries := n.log.Entries(nextIdx)

		n.transport.Send(Msg{
			Type:    MsgHeartbeat,
			From:    n.ID,
			To:      peer,
			Term:    term,
			Index:   commitIdx,
			Entries: entries,
		})
	}
}

// handleHeartbeat processes a heartbeat from the leader. It appends any
// piggybacked entries to the local log and resets the election timer.
func (n *Node) handleHeartbeat(msg Msg, electionTimer *time.Timer) {
	n.mu.Lock()
	n.leader = msg.From
	n.role = Follower
	n.mu.Unlock()

	electionTimer.Reset(n.randomElectionTimeout())

	// Append any entries included with the heartbeat
	if len(msg.Entries) > 0 {
		for _, e := range msg.Entries {
			existing, ok := n.log.Get(e.Index)
			if !ok {
				n.log.Append(e)
			} else if existing.Term != e.Term {
				n.log.Append(e)
			}
		}
	}

	n.transport.Send(Msg{
		Type:    MsgHeartbeatResp,
		From:    n.ID,
		To:      msg.From,
		Term:    msg.Term,
		Index:   n.log.LastIndex(),
		Success: true,
	})
}

// handleHeartbeatResp processes a heartbeat response from a follower.
// The leader uses the follower's reported last log index to update nextIndex
// so subsequent heartbeats carry any entries the follower is missing.
func (n *Node) handleHeartbeatResp(msg Msg) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.role != Leader {
		return
	}
	n.matchIndex[msg.From] = msg.Index
	n.nextIndex[msg.From] = msg.Index + 1
}

func (n *Node) handleAppendEntries(msg Msg, electionTimer *time.Timer) {
	n.mu.Lock()
	n.leader = msg.From
	n.role = Follower
	n.mu.Unlock()

	electionTimer.Reset(n.randomElectionTimeout())

	for _, e := range msg.Entries {
		n.log.Append(e)
	}

	// Advance commit index based on the leader's commit index
	leaderCommit := msg.Index
	if leaderCommit > 0 {
		lastIdx := n.log.LastIndex()
		commitIdx := leaderCommit
		if commitIdx > lastIdx {
			commitIdx = lastIdx
		}
		n.advanceCommitIndex(commitIdx)
	}

	n.transport.Send(Msg{
		Type:    MsgAppendResp,
		From:    n.ID,
		To:      msg.From,
		Term:    msg.Term,
		Index:   n.log.LastIndex(),
		Success: true,
	})
}

func (n *Node) handleAppendResp(msg Msg) {
	if !msg.Success {
		return
	}

	n.mu.Lock()
	if n.role != Leader {
		n.mu.Unlock()
		return
	}
	n.matchIndex[msg.From] = msg.Index
	n.nextIndex[msg.From] = msg.Index + 1

	// Check if we can advance commit index
	for idx := n.commitIndex + 1; idx <= n.log.LastIndex(); idx++ {
		replicaCount := 1 // count self
		for _, peer := range n.peers {
			if n.matchIndex[peer] >= idx {
				replicaCount++
			}
		}
		majority := (len(n.peers)+1)/2 + 1
		if replicaCount >= majority {
			n.mu.Unlock()
			n.advanceCommitIndex(idx)
			n.mu.Lock()
		}
	}
	n.mu.Unlock()
}

func (n *Node) handleReadIndex(msg Msg) {
	n.mu.RLock()
	role := n.role
	commitIdx := n.commitIndex
	n.mu.RUnlock()

	if role != Leader {
		return
	}

	n.transport.Send(Msg{
		Type:  MsgReadIndexResp,
		From:  n.ID,
		To:    msg.From,
		Index: commitIdx,
	})
}

func (n *Node) handleReadIndexResp(msg Msg) {
	ch := n.getReadIndexWaiter()
	if ch != nil {
		select {
		case ch <- msg.Index:
		default:
		}
	}
}
