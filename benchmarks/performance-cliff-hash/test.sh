#!/bin/sh
# Test for performance degradation in hash map at scale.
# The bug: hashCode() uses category instead of SKU, so all products
# in the same category collide.  With only 10 categories, 10 000 items
# land in ~10 buckets — max chain length explodes.

set -e

FAIL=0

echo "=== Performance Cliff Hash Map Test ==="

# Test 1: Small dataset (should always pass — even bad hash is fast with few items)
echo "--- Small dataset (100 items) ---"
perftest bench-small
echo "Small dataset: OK"

# Test 2: Distribution analysis — the reliable, deterministic check.
# With proper hashing (SKU-based), 10 000 items across 1024+ buckets
# should have a max chain length well under 50.
# With the buggy hash (category-based, only 10 distinct values),
# max chain length will be ~1000.
echo "--- Hash distribution analysis ---"
perftest analyze > /tmp/hash_dist.txt 2>&1
cat /tmp/hash_dist.txt

MAX_CHAIN=$(grep "Max chain length:" /tmp/hash_dist.txt | awk '{print $NF}')
echo "Detected max chain length: $MAX_CHAIN"

if [ -z "$MAX_CHAIN" ]; then
    echo "FAIL: Could not parse max chain length from analyze output"
    FAIL=1
elif [ "$MAX_CHAIN" -gt 50 ]; then
    echo "FAIL: Max chain length $MAX_CHAIN exceeds 50 — severe hash collisions"
    echo "      This indicates hashCode() is not distributing items across buckets"
    FAIL=1
else
    echo "OK: Max chain length $MAX_CHAIN is within acceptable bounds"
fi

rm -f /tmp/hash_dist.txt

if [ $FAIL -eq 0 ]; then
    echo "PASS: Hash map distributes items evenly across buckets"
    exit 0
else
    echo "FAIL: Hash map has severe bucket collisions from bad hashCode()"
    exit 1
fi
