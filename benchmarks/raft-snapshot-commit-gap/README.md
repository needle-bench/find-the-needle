# raft-snapshot-commit-gap

## Project

A Go implementation of a simplified Raft consensus protocol for a 3-node cluster with leader election, log replication, snapshotting, and crash recovery. The system includes a key-value state machine that tracks apply counts to detect duplicate transitions.

## Symptoms

After the sequence: (1) normal writes replicated to all nodes, (2) leader takes a snapshot, (3) snapshot sent to a slow follower, (4) leader changes and follower restarts -- the follower's state machine contains duplicate entries. Keys that should have been applied exactly once are applied twice, as revealed by `raft-snapshot check` and the apply-count tracking in the state machine.

Each step works correctly in isolation. Normal replication, snapshotting, snapshot installation, and crash recovery each behave correctly on their own. Only the specific 4-step ordering triggers the bug.

## Bug description

The InstallSnapshot RPC handler has two omissions that interact with the crash recovery path. The handler correctly updates lastApplied but fails to maintain consistency with another piece of volatile state and does not properly clean up superseded data structures. After a crash-recovery cycle, the node's recovery logic derives its replay starting point from the stale volatile state, causing log entries already baked into the snapshot to be re-applied to the state machine.

The log module contains what appears to be an off-by-one in `CompactUpTo` (using `<` where `<=` might seem correct), but this follows the Raft paper's convention for retaining the snapshot boundary entry for consistency checks -- it is not the bug.

## Difficulty

Extreme

## Expected turns

30-50
