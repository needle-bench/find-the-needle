package main

import (
	"fmt"
	"strings"
	"sync"
)

// StateMachine is a simple key-value store that tracks how many times
// each key has been set. This allows detection of duplicate applies.
type StateMachine struct {
	mu     sync.Mutex
	data   map[string]string
	counts map[string]int // how many times each key was set
}

func NewStateMachine() *StateMachine {
	return &StateMachine{
		data:   make(map[string]string),
		counts: make(map[string]int),
	}
}

// Apply processes a command in the form "SET key value".
// Returns an error if the command is malformed.
func (sm *StateMachine) Apply(command string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	parts := strings.SplitN(command, " ", 3)
	if len(parts) != 3 || parts[0] != "SET" {
		return fmt.Errorf("invalid command: %q", command)
	}

	key, value := parts[1], parts[2]
	sm.data[key] = value
	sm.counts[key]++
	return nil
}

// Get returns the value for a key and whether it exists.
func (sm *StateMachine) Get(key string) (string, bool) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	v, ok := sm.data[key]
	return v, ok
}

// GetCount returns how many times a key has been applied.
func (sm *StateMachine) GetCount(key string) int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.counts[key]
}

// Snapshot serializes the state machine's current data (not the counts).
// The counts are intentionally excluded so that duplicate applies after
// snapshot restoration are still observable.
func (sm *StateMachine) Snapshot() map[string]string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	snap := make(map[string]string, len(sm.data))
	for k, v := range sm.data {
		snap[k] = v
	}
	return snap
}

// Restore replaces the state machine's data from a snapshot.
// Counts are reset to 1 for each key in the snapshot since snapshot
// represents committed, already-applied state.
func (sm *StateMachine) Restore(data map[string]string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.data = make(map[string]string, len(data))
	sm.counts = make(map[string]int, len(data))
	for k, v := range data {
		sm.data[k] = v
		sm.counts[k] = 1
	}
}

// AllCounts returns a copy of the apply-count map.
func (sm *StateMachine) AllCounts() map[string]int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	out := make(map[string]int, len(sm.counts))
	for k, v := range sm.counts {
		out[k] = v
	}
	return out
}
