package main

import (
	"fmt"
	"net/http"
	"time"

	"linearizability-stale-read/raft"
)

const (
	BasePort = 9100
	NumNodes = 3
)

// Cluster manages a set of raft nodes running in-process.
type Cluster struct {
	nodes     []*raft.Node
	transport *raft.Transport
	servers   []*http.Server
}

func NewCluster() *Cluster {
	ids := make([]int, NumNodes)
	for i := range ids {
		ids[i] = i
	}

	transport := raft.NewTransport(ids)
	nodes := make([]*raft.Node, NumNodes)

	for i := 0; i < NumNodes; i++ {
		peers := make([]int, 0, NumNodes-1)
		for j := 0; j < NumNodes; j++ {
			if j != i {
				peers = append(peers, j)
			}
		}
		nodes[i] = raft.NewNode(i, peers, transport)
	}

	return &Cluster{
		nodes:     nodes,
		transport: transport,
	}
}

func (c *Cluster) Start() {
	for _, n := range c.nodes {
		n.Start()
	}

	// Start HTTP servers
	c.servers = make([]*http.Server, NumNodes)
	for i, n := range c.nodes {
		handler := NewHandler(n, c)
		srv := &http.Server{
			Addr:    fmt.Sprintf(":%d", BasePort+i),
			Handler: handler,
		}
		c.servers[i] = srv
		go srv.ListenAndServe()
	}
}

func (c *Cluster) Stop() {
	for _, n := range c.nodes {
		n.Stop()
	}
	for _, s := range c.servers {
		s.Close()
	}
}

func (c *Cluster) Port(nodeID int) int {
	return BasePort + nodeID
}

func (c *Cluster) Node(id int) *raft.Node {
	return c.nodes[id]
}

func (c *Cluster) Transport() *raft.Transport {
	return c.transport
}

// WaitForLeader waits until a leader is elected, returns the leader's node ID.
func (c *Cluster) WaitForLeader(timeout time.Duration) (int, error) {
	deadline := time.After(timeout)
	for {
		for _, n := range c.nodes {
			if n.Role() == raft.Leader {
				return n.ID, nil
			}
		}
		select {
		case <-deadline:
			return -1, fmt.Errorf("no leader elected within %v", timeout)
		case <-time.After(50 * time.Millisecond):
		}
	}
}
