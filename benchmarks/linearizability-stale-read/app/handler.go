package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"linearizability-stale-read/raft"
)

// Handler serves HTTP requests for a single node's KV store.
type Handler struct {
	node    *raft.Node
	cluster *Cluster
}

type WriteRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type ReadResponse struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	Found bool   `json:"found"`
	Node  int    `json:"node"`
}

type StatusResponse struct {
	Node        int    `json:"node"`
	Role        string `json:"role"`
	Term        uint64 `json:"term"`
	Leader      int    `json:"leader"`
	CommitIndex uint64 `json:"commit_index"`
	AppliedIndex uint64 `json:"applied_index"`
}

func NewHandler(node *raft.Node, cluster *Cluster) *Handler {
	return &Handler{node: node, cluster: cluster}
}

// ServeHTTP routes requests to the appropriate handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case r.Method == "GET" && r.URL.Path == "/status":
		h.handleStatus(w, r)
	case r.Method == "GET" && r.URL.Path == "/read":
		h.handleRead(w, r)
	case r.Method == "POST" && r.URL.Path == "/write":
		h.handleWrite(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (h *Handler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	role := h.node.Role()
	roleStr := "follower"
	if role == raft.Leader {
		roleStr = "leader"
	} else if role == raft.Candidate {
		roleStr = "candidate"
	}

	resp := StatusResponse{
		Node:        h.node.ID,
		Role:        roleStr,
		Term:        h.node.Term(),
		Leader:      h.node.Leader(),
		CommitIndex: h.node.CommitIndex(),
		AppliedIndex: h.node.AppliedIndex(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleRead serves a read request. Reads are served from local state
// for low latency — the node's KV map is populated by the apply loop
// as committed entries are processed.
func (h *Handler) handleRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "missing key parameter", http.StatusBadRequest)
		return
	}

	value, found := h.node.Read(key)

	resp := ReadResponse{
		Key:   key,
		Value: value,
		Found: found,
		Node:  h.node.ID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) handleWrite(w http.ResponseWriter, r *http.Request) {
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Key == "" {
		http.Error(w, "missing key", http.StatusBadRequest)
		return
	}

	// If this node is not the leader, redirect to the leader
	if h.node.Role() != raft.Leader {
		leaderID := h.node.Leader()
		if leaderID < 0 {
			http.Error(w, "no leader available", http.StatusServiceUnavailable)
			return
		}
		port := h.cluster.Port(leaderID)
		http.Redirect(w, r, fmt.Sprintf("http://127.0.0.1:%d/write", port), http.StatusTemporaryRedirect)
		return
	}

	ok := h.node.Write(req.Key, req.Value)
	if !ok {
		http.Error(w, "write failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
