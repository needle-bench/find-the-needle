#!/bin/sh
# Test for duplicate state machine transitions after snapshot install + restart.
# A correct implementation must never apply the same log entry twice.
#
# Exit 0 = bug is fixed (pass)
# Exit 1 = bug still present (fail)

set -e

FAIL=0

echo "=== Raft Snapshot Commit-Gap Test ==="

# Rebuild the binary to pick up any code changes
echo "Building..."
cd /app/app && go build -o /usr/local/bin/raft-snapshot . 2>&1
cd /app

# Run the check multiple times for confidence
for round in 1 2 3; do
    echo "--- Round $round ---"
    if ! raft-snapshot check; then
        echo "FAIL: Duplicate applies detected in round $round"
        FAIL=1
        break
    fi
    echo "Round $round: OK"
done

if [ $FAIL -eq 0 ]; then
    echo "PASS: No duplicate state machine transitions across all rounds"
    exit 0
else
    echo ""
    echo "FAIL: State machine has duplicate entries after snapshot+restart sequence"
    echo ""
    echo "The 4-step reproduction:"
    echo "  1. Leader writes entries 1-10, replicates to all nodes"
    echo "  2. Leader takes snapshot at index 10"
    echo "  3. Leader sends snapshot to slow follower (node3)"
    echo "  4. Leader changes, node3 restarts, new leader replicates"
    echo ""
    echo "Run 'raft-snapshot simulate' for detailed trace."
    exit 1
fi
