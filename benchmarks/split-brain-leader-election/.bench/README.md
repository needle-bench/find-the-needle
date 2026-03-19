# split-brain-leader-election

## Difficulty
Hard

## Source
Community-submitted

## Environment
Go 1.22, Alpine Linux

## The bug
The quorum check in `app/cluster/node.go` uses `finalVotes >= totalNodes/2` instead of `finalVotes >= totalNodes/2 + 1`. For a 5-node cluster, `5/2 = 2` (integer division), so a node with only 2 votes can become leader. When a network partition splits the cluster into groups of 2 and 3, both sides can achieve 2 votes and elect a leader simultaneously, violating the single-leader invariant.

## Why Hard
Requires understanding distributed consensus, quorum arithmetic, and how integer division interacts with majority thresholds. The agent must reason about partition scenarios and vote counts across network splits. The off-by-one in the quorum check is subtle because it only manifests under specific partition topologies.

## Expected fix
Change the quorum threshold to `totalNodes/2 + 1` (strict majority) so that only a partition with more than half the nodes can elect a leader.

## Pinned at
Anonymized snapshot, original repo not disclosed
