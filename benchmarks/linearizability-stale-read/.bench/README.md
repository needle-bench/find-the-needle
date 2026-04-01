# linearizability-stale-read

## Difficulty
Extreme

## Source
Synthetic benchmark inspired by real Raft linearizable-read bugs (etcd#7625, TiKV#3415)

## Environment
Go 1.22, Alpine Linux

## The bug
A 3-node distributed KV store implements Raft-like consensus with an async apply channel. The system claims linearizability but has two compounding bugs:

1. **Read handler serves local state directly** (`handler.go`): The HTTP read handler calls `node.Read()` which returns whatever is in the local KV map without any linearizability guarantee. A `ReadLinearizable()` method exists that implements the read-index protocol (ask the leader for the current commit index, wait for local applied index to catch up) but is never called.

2. **Heartbeat catch-up does not advance commit index** (`raft/node.go`): When a follower receives entries piggybacked on a heartbeat (the catch-up path after partition heal), `handleHeartbeat` appends them to the log but never calls `advanceCommitIndex`. The entries sit in the log forever without being applied to the state machine. The `handleAppendEntries` path correctly advances the commit index, masking the bug during normal (non-partition) operation.

Together, these bugs mean: after a partition heals, a client can write K=1 to the leader (committed by majority), then read K from a formerly-partitioned follower and get the stale value (empty/not-found), violating linearizability.

## Why Extreme
- Requires understanding Raft consensus semantics: commit index, apply channel, read-index protocol
- The bug spans two files and two layers (HTTP handler + consensus protocol)
- Fix #1 alone (switching to ReadLinearizable) is insufficient -- the read will timeout because fix #2 is also needed
- Fix #2 alone (advancing commit index in handleHeartbeat) is insufficient -- reads still bypass the linearizability check
- Red herring: the heartbeat interval (200ms) appears higher than the election timeout base (150ms), which looks like a classic misconfiguration, but is actually correct due to jitter making the effective election timeout 300-600ms
- The `handleAppendEntries` path has the correct commit-index advancement, creating a false sense that the protocol is complete
- The apply channel is async, adding another layer of indirection between commit and visibility

## Expected fix
Two changes across two files:

1. `app/handler.go`: Change `handleRead` to call `node.ReadLinearizable(key)` instead of `node.Read(key)`
2. `app/raft/node.go`: In `handleHeartbeat`, after appending entries, advance the commit index based on the leader's commit index (same pattern as `handleAppendEntries`)

## Expected turns
20-40

## Pinned at
Synthetic (not derived from a specific upstream commit)
