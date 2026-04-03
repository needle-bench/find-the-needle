#!/usr/bin/env bash
# Run all 40 benchmarks for a single model across applicable arms.
# Usage: bash run_model.sh <model> <arm_count> [needle_id]
set -uo pipefail
cd "$(dirname "$0")"

MODEL="${1:?Usage: $0 <model> <arm_count> [needle_id]}"
ARM_COUNT="${2:?Usage: $0 <model> <arm_count> [needle_id]}"
NEEDLE_ID="${3:-}"

BENCHMARKS=(
  api-version-field-drop auth-bypass-path-traversal bidi-override-injection
  cache-stale-invalidation compiler-macro-expansion data-corruption-concurrent-write
  deadlock-transfer encoding-mojibake goroutine-leak-handler
  graphql-dataloader-per-request haystack-boot haystack-mint
  import-cycle-startup k8s-assume-cache-silent-drop k8s-scheduler-shutdown-deadlock
  kernel-panic-ioctl linearizability-stale-read memory-leak-event-listener
  missing-input-validation nginx-upstream-port-mismatch null-pointer-config
  off-by-one-array-slice off-by-one-pagination performance-cliff-hash
  postgres-migration-schema-drift race-condition-counter raft-snapshot-commit-gap
  rate-limit-bypass-header relaxed-ordering-ringbuf retry-storm-duplicate-transfer
  silent-data-corruption split-brain-leader-election sql-injection-search
  ssrf-allowlist-port-confusion timezone-scheduling timing-attack-comparison
  tls-chain-ordering-strict type-coercion-comparison wal-fsync-ghost-ack
  wrong-operator-discount
)

PASS=0; FAIL=0; SKIP=0

for bench in "${BENCHMARKS[@]}"; do
  for arm_spec in native "kernel" "kernel-cpu"; do
    case "$arm_spec" in
      native)
        [ "$ARM_COUNT" != "3" ] && continue
        arm_label="native"
        arm_flags="--arm native --local"
        ;;
      kernel)
        arm_label="kernel"
        arm_flags="--arm kernel --local"
        ;;
      kernel-cpu)
        arm_label="kernel-cpu"
        arm_flags="--arm kernel --driver cpu --local"
        ;;
    esac

    score_file="runs/${MODEL}-${arm_label}/${bench}.score.json"
    if [ -f "$score_file" ]; then
      SKIP=$((SKIP + 1))
      continue
    fi

    # shellcheck disable=SC2086
    if ostk bench "$bench" --model "$MODEL" $arm_flags --docker 2>&1 | tail -1; then
      PASS=$((PASS + 1))
    else
      FAIL=$((FAIL + 1))
    fi
  done
done

# Verify AC
expected=$((40 * ARM_COUNT))
actual=$(find "runs/" -path "runs/${MODEL}-*/*.score.json" 2>/dev/null | wc -l | tr -d ' ')
echo ""
echo "=== ${MODEL} DONE: ${actual}/${expected} scores (pass=$PASS fail=$FAIL skip=$SKIP) ==="

if [ -n "$NEEDLE_ID" ] && [ "$actual" -ge "$expected" ]; then
  ostk needle close "$NEEDLE_ID" 2>/dev/null && echo "  →${NEEDLE_ID} closed" || true
fi
