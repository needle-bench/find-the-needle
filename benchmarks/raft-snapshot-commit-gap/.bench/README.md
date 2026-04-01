# raft-snapshot-commit-gap

## Difficulty
Extreme

## Source
Community-submitted benchmark, synthetic Raft implementation

## Environment
Go 1.22, Alpine Linux

## The bug
The `InstallSnapshot` handler in `snapshot.go` sets `lastApplied = snapshot.lastIncludedIndex` but omits two critical updates: (1) it never sets `commitIndex = max(commitIndex, snapshot.lastIncludedIndex)`, and (2) it never calls `CompactUpTo` to discard log entries superseded by the snapshot. After the sequence (writes -> snapshot -> install on follower -> restart), the recovery path in `Restart()` sees `commitIndex=0` (never updated by InstallSnapshot), and because `savedCommitIndex < snapshotAppliedTo`, it sets `lastApplied = savedCommitIndex = 0`. The old log entries (still present, never compacted) are then re-applied on top of the snapshot-restored state machine, causing every key to be applied twice.

## Why Extreme
- The bug spans 3 files (snapshot.go handler, log.go compaction, raft.go recovery) and requires understanding how they interact across the snapshot-install-restart lifecycle.
- Each individual step works correctly in isolation -- only the specific 4-step ordering triggers the failure.
- A red herring in log.go's `CompactUpTo` uses strict `<` comparison that looks like an off-by-one but is correct per the Raft paper's indexing convention.
- The recovery path in `Restart()` has a subtle branch where `savedCommitIndex < snapshotAppliedTo` causes `lastApplied` to be dragged down to `commitIndex=0`, but this branch is only reachable when InstallSnapshot failed to update commitIndex.
- Requires deep understanding of the Raft paper's volatile vs. persistent state distinction and snapshot semantics.

## Expected fix
In `snapshot.go`'s `InstallSnapshot` handler, add two lines after setting `lastApplied`:
1. `n.raftLog.CompactUpTo(snap.LastIncludedIndex)` -- discard superseded log entries
2. `if snap.LastIncludedIndex > n.commitIndex { n.commitIndex = snap.LastIncludedIndex }` -- update commitIndex to match snapshot

## Pinned at
Synthetic implementation, not derived from an external repository
