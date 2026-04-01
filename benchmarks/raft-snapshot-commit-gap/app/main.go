package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: raft-snapshot <simulate|check>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  simulate  - Run the snapshot commit-gap reproduction")
		fmt.Fprintln(os.Stderr, "  check     - Check if duplicate applies occur after snapshot+restart")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "simulate":
		runSimulation(os.Stdout)
	case "check":
		ok := runCheck(os.Stdout)
		if !ok {
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

// buildScenario sets up and runs the 4-step reproduction sequence.
// Returns node3 so callers can inspect its state machine.
func buildScenario(w io.Writer) *RaftNode {
	// Create a 3-node cluster: nodes 1, 2, 3
	node1 := NewRaftNode(1, []int{2, 3})
	node2 := NewRaftNode(2, []int{1, 3})
	node3 := NewRaftNode(3, []int{1, 2})

	node1.SetLogWriter(w)
	node2.SetLogWriter(w)
	node3.SetLogWriter(w)

	// ---- Step 1: Normal writes with node1 as leader ----
	// Node1 is leader in term 1. All 3 nodes are up.
	// Entries 1-10 are proposed and replicated to all nodes.
	// However, only entries replicated to node1+node2 are committed (quorum).
	// Node3 receives entries (log replication) but never sees the commit.
	node1.BecomeLeader(1)
	node2.BecomeFollower(1, 1)
	node3.BecomeFollower(1, 1)

	for i := 1; i <= 10; i++ {
		cmd := fmt.Sprintf("SET key%d value%d", i, i)
		node1.ProposeEntry(cmd)
	}

	// Replicate to both node2 and node3
	prevIdx, prevTerm, entries, leaderCommit := node1.PrepareAppendEntries(2)
	node2.AppendEntries(1, 1, prevIdx, prevTerm, entries, leaderCommit)

	prevIdx, prevTerm, entries, leaderCommit = node1.PrepareAppendEntries(3)
	node3.AppendEntries(1, 1, prevIdx, prevTerm, entries, leaderCommit)

	// Commit on leader (quorum: node1 + node2)
	node1.CommitEntries(10)
	node1.UpdateNextIndex(2, 10)
	node1.UpdateNextIndex(3, 10)

	// Send commit update to node2 (fast follower)
	prevIdx, prevTerm, entries, leaderCommit = node1.PrepareAppendEntries(2)
	node2.AppendEntries(1, 1, prevIdx, prevTerm, entries, leaderCommit)

	// Node3 does NOT receive the commit update — it's slow / partitioned
	// At this point: node3 has entries 1-10 in its log, but commitIndex=0

	// ---- Step 2: Leader takes snapshot ----
	// Leader snapshots its committed state (entries 1-10)
	snap := node1.TakeSnapshot()

	// ---- Step 3: Send snapshot to slow follower (node3) ----
	// Leader decides node3 is too far behind (missed commit updates)
	// and sends the snapshot instead of replaying commits.
	// After this: node3.lastApplied=10, node3.commitIndex=0 (BUG)
	// Node3's log still contains entries 1-10 (not compacted)
	node3.InstallSnapshot(snap)

	// ---- Step 4: Leader change + follower restart ----
	// Node3 crashes and restarts. On recovery:
	//   - State machine is restored from snapshot (keys 1-10)
	//   - commitIndex was 0 before crash (never updated by InstallSnapshot)
	//   - So lastApplied is set to commitIndex=0 instead of snapshot index=10
	//   - Log entries 1-10 are still present (never compacted)
	node3.Restart()

	// Node2 becomes leader in term 2
	node2.BecomeLeader(2)

	// New leader writes entries 11-13
	for i := 11; i <= 13; i++ {
		cmd := fmt.Sprintf("SET key%d value%d", i, i)
		node2.ProposeEntry(cmd)
	}
	node2.CommitEntries(13)

	// New leader replicates to node3.
	// Node3's log has entries at indices 1-10 from term 1 and the snapshot
	// boundary at index 10. The leader sends entries 11-13 with prevLogIndex=10.
	prevIdx, prevTerm, entries, leaderCommit = node2.PrepareAppendEntries(3)
	node3.AppendEntries(2, 2, prevIdx, prevTerm, entries, leaderCommit)

	return node3
}

// runSimulation runs the full 4-step reproduction scenario and prints
// detailed output about what happens at each stage.
func runSimulation(w io.Writer) {
	fmt.Fprintln(w, "=== Raft Snapshot Commit-Gap Reproduction ===")
	fmt.Fprintln(w)

	node3 := buildScenario(w)

	// Check for duplicates
	fmt.Fprintln(w, "\n=== Duplicate Detection ===")
	counts := node3.stateMachine.AllCounts()
	duplicates := 0
	for key, count := range counts {
		if count > 1 {
			fmt.Fprintf(w, "  DUPLICATE: %s applied %d times\n", key, count)
			duplicates++
		}
	}
	if duplicates > 0 {
		fmt.Fprintf(w, "\nFAIL: %d keys were applied more than once\n", duplicates)
	} else {
		fmt.Fprintln(w, "\nPASS: No duplicate applies detected")
	}
}

// runCheck runs the reproduction and returns true if no duplicates were found.
func runCheck(w io.Writer) bool {
	node3 := buildScenario(io.Discard)

	// Check for duplicate applies
	counts := node3.stateMachine.AllCounts()
	duplicates := 0
	for key, count := range counts {
		if count > 1 {
			fmt.Fprintf(w, "DUPLICATE: %s applied %d times\n", key, count)
			duplicates++
		}
	}

	if duplicates > 0 {
		fmt.Fprintf(w, "FAIL: %d keys had duplicate state machine transitions\n", duplicates)
		return false
	}

	// Verify all 13 keys are present with correct values
	for i := 1; i <= 13; i++ {
		key := fmt.Sprintf("key%d", i)
		val, exists := node3.stateMachine.Get(key)
		if !exists {
			fmt.Fprintf(w, "FAIL: %s missing from state machine\n", key)
			return false
		}
		expected := fmt.Sprintf("value%d", i)
		if val != expected {
			fmt.Fprintf(w, "FAIL: %s = %q, expected %q\n", key, val, expected)
			return false
		}
	}

	fmt.Fprintln(w, "PASS: All 13 keys present, no duplicate applies")
	return true
}
