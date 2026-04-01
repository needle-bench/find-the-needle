#!/bin/sh
# Test for memory ordering correctness in SPSC ring buffer.
#
# This test builds the ring buffer with ThreadSanitizer (-fsanitize=thread)
# and runs the producer/consumer stress test. TSan detects data races caused
# by incorrect memory ordering on atomic operations.
#
# Exit 0 = no races detected (bug is fixed)
# Exit 1 = TSan reports data races (bug present)

set -e

FAIL=0
APP_DIR=/app

echo "=== SPSC Ring Buffer Memory Ordering Test ==="

# --- Build with ThreadSanitizer ---
echo "--- Building with ThreadSanitizer ---"
cd "$APP_DIR"
make clean 2>&1
make tsan 2>&1

echo ""
echo "--- Running ThreadSanitizer stress test ---"

# TSan writes diagnostics to stderr and returns non-zero on race detection.
# We capture both and check the exit code.
TSAN_LOG=/tmp/tsan_output.log

# Run with fewer items under TSan (still enough to trigger races)
set +e
./ringbuf_test_tsan 262144 > /tmp/tsan_stdout.log 2>"$TSAN_LOG"
TSAN_EXIT=$?
set -e

cat /tmp/tsan_stdout.log

# Check for ThreadSanitizer warnings
if grep -q "ThreadSanitizer" "$TSAN_LOG" || [ $TSAN_EXIT -ne 0 ]; then
    echo ""
    echo "--- ThreadSanitizer output ---"
    cat "$TSAN_LOG"
    echo ""
    echo "FAIL: ThreadSanitizer detected data race(s)"
    echo "      The atomic operations use incorrect memory ordering."
    echo "      Review the memory_order_* parameters on atomic loads/stores"
    echo "      in the producer (ringbuf_push) and consumer (ringbuf_pop)."
    FAIL=1
else
    echo ""
    echo "OK: No data races detected by ThreadSanitizer"
fi

# --- Also verify functional correctness with a normal build ---
echo ""
echo "--- Building optimized (non-TSan) for correctness check ---"
make clean 2>&1
make 2>&1

echo "--- Running correctness test ---"
set +e
./ringbuf_test 1048576 > /tmp/normal_stdout.log 2>&1
NORMAL_EXIT=$?
set -e

cat /tmp/normal_stdout.log

if [ $NORMAL_EXIT -ne 0 ]; then
    echo "FAIL: Functional correctness test failed"
    FAIL=1
else
    echo "OK: Functional correctness verified"
fi

# --- Cleanup ---
rm -f /tmp/tsan_output.log /tmp/tsan_stdout.log /tmp/normal_stdout.log

echo ""
if [ $FAIL -eq 0 ]; then
    echo "PASS: All tests passed — no races, correct output"
    exit 0
else
    echo "FAIL: Memory ordering bug detected"
    exit 1
fi
