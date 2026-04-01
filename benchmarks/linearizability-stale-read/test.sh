#!/bin/sh
# Linearizability test for the distributed KV store.
#
# This test verifies that reads are linearizable after a network partition heals.
# A correct implementation must guarantee that once a write is acknowledged by
# the leader, any subsequent read from ANY node returns the written value.
#
# Steps:
#   1. Start 3-node cluster, wait for leader election
#   2. Partition one follower from the cluster
#   3. Write K=1 to the leader (committed by majority: leader + remaining follower)
#   4. Heal the partition
#   5. Read K from the formerly-partitioned follower
#   6. Assert K=1 (linearizable read)
#
# Expected: FAIL on buggy code (stale read), PASS after fix.

set -e

echo "=== Linearizability Stale-Read Test ==="

# Rebuild in case source was modified
echo "Building..."
cd /app/app && go build -o /usr/local/bin/kv-cluster . 2>&1
cd /app

FAIL=0

for round in 1 2 3; do
    echo "--- Round $round ---"
    if ! kv-cluster check; then
        echo "FAIL: Linearizability violation detected in round $round"
        FAIL=1
        break
    fi
    echo "Round $round: OK"
done

if [ $FAIL -eq 0 ]; then
    echo "PASS: All rounds passed — reads are linearizable"
    exit 0
else
    echo "FAIL: Reads are not linearizable — stale data returned after partition heal"
    exit 1
fi
