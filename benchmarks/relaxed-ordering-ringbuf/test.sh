#!/bin/sh
# Test for memory ordering correctness in SPSC ring buffer.
#
# This test builds the ring buffer with ThreadSanitizer (-fsanitize=thread)
# and runs the producer/consumer stress test. TSan detects data races caused
# by incorrect memory ordering on atomic operations.
#
# On platforms where TSan cannot run (e.g. ARM64 Docker without
# --security-opt seccomp=unconfined), we fall back to a multi-run
# functional correctness test that detects races via data corruption.
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

# Detect TSan crash due to personality/ASLR issue (ARM64 containers)
TSAN_CRASHED=0
if [ $TSAN_EXIT -ne 0 ] && grep -q "tsan_platform_linux" "$TSAN_LOG" 2>/dev/null; then
    TSAN_CRASHED=1
fi

if [ $TSAN_CRASHED -eq 1 ]; then
    echo ""
    echo "NOTE: ThreadSanitizer cannot run in this container (ASLR/personality restriction)."
    echo "      Falling back to multi-run functional correctness test."
    echo ""

    # Fall back to repeated functional correctness tests.
    # On ARM64 (weak memory model), relaxed atomics cause data corruption
    # that manifests reliably over multiple runs with the optimized build.
    echo "--- Building optimized (non-TSan) for correctness check ---"
    make clean 2>&1
    make 2>&1

    echo "--- Running multi-iteration correctness test ---"
    CORR_FAIL=0
    for i in 1 2 3 4 5; do
        set +e
        ./ringbuf_test 1048576 > /tmp/normal_stdout_$i.log 2>&1
        RUN_EXIT=$?
        set -e
        if [ $RUN_EXIT -ne 0 ]; then
            echo "  Run $i: FAIL"
            cat /tmp/normal_stdout_$i.log
            CORR_FAIL=1
        else
            echo "  Run $i: OK"
        fi
        rm -f /tmp/normal_stdout_$i.log
    done

    if [ $CORR_FAIL -ne 0 ]; then
        echo ""
        echo "FAIL: Data corruption detected — memory ordering bug present"
        FAIL=1
    else
        echo ""
        echo "OK: All runs passed — no data corruption detected"
    fi
else
    # Normal TSan path (x86 or containers with seccomp=unconfined)
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
