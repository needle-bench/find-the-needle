package raft

import "sync"

// Msg is a message passed between nodes.
type Msg struct {
	Type     MsgType
	From     int
	To       int
	Term     uint64
	Entries  []Entry
	Index    uint64 // used for AppendEntries match, commit index, read-index queries
	Success  bool
	LeaderID int
}

type MsgType int

const (
	MsgVoteReq MsgType = iota
	MsgVoteResp
	MsgAppendEntries
	MsgAppendResp
	MsgHeartbeat
	MsgHeartbeatResp
	MsgReadIndex
	MsgReadIndexResp
)

// Transport delivers messages between nodes with optional partition simulation.
type Transport struct {
	mu         sync.RWMutex
	mailboxes  map[int]chan Msg
	partitions map[[2]int]bool // directional partition: [from, to] -> blocked
}

func NewTransport(nodeIDs []int) *Transport {
	t := &Transport{
		mailboxes:  make(map[int]chan Msg),
		partitions: make(map[[2]int]bool),
	}
	for _, id := range nodeIDs {
		t.mailboxes[id] = make(chan Msg, 256)
	}
	return t
}

func (t *Transport) Send(msg Msg) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	key := [2]int{msg.From, msg.To}
	if t.partitions[key] {
		return // message dropped by partition
	}

	ch, ok := t.mailboxes[msg.To]
	if !ok {
		return
	}
	select {
	case ch <- msg:
	default:
		// mailbox full, drop message
	}
}

func (t *Transport) Recv(nodeID int) <-chan Msg {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.mailboxes[nodeID]
}

// Partition blocks messages in both directions between a and b.
func (t *Transport) Partition(a, b int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.partitions[[2]int{a, b}] = true
	t.partitions[[2]int{b, a}] = true
}

// Heal removes all partitions.
func (t *Transport) Heal() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.partitions = make(map[[2]int]bool)
}
