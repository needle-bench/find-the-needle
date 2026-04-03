#!/usr/bin/env bash
# Full matrix — 10 models in parallel.
# Usage: nohup bash run_matrix_parallel.sh 2>&1 | tee matrix.log &
set -uo pipefail
cd "$(dirname "$0")"

PARALLEL=10

# model:needle_id:arm_count
MODELS=(
  "claude-haiku-4-5:1043:3"
  "claude-sonnet-4-6:1044:3"
  "claude-opus-4-6:1045:3"
  "gemini-2.5-flash:1046:3"
  "gemini-2.5-pro:1047:3"
  "gemini-3-flash-preview:1048:3"
  "gemini-3.1-pro-preview:1049:3"
  "devstral-2512:1050:3"
  "devstral-medium:1051:3"
  "devstral-small-latest:1052:3"
  "kimi-k2.5:1053:3"
  "codestral-2508:1054:2"
  "gpt-4.1:1055:2"
  "gpt-5-codex:1056:2"
  "o3:1057:2"
  "o4-mini:1058:2"
  "grok-3:1059:2"
  "grok-3-fast:1060:2"
  "grok-3-mini:1061:2"
  "grok-4:1062:2"
  "grok-4-fast:1063:2"
  "grok-4.1-fast:1064:2"
  "grok-4.20:1065:2"
  "grok-code-fast-1:1066:2"
  "deepseek-r1:1067:2"
  "deepseek-r1-0528:1068:2"
  "deepseek-v3.2:1069:2"
  "qwen3-coder:1070:2"
  "qwen3-coder-flash:1071:2"
  "qwen3-coder-plus:1072:2"
  "llama-4-maverick:1073:2"
)

echo "=== needle-bench full matrix (${PARALLEL} parallel) ==="
echo "=== started $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
echo "=== ${#MODELS[@]} models × 40 benchmarks ==="
echo ""

# Run in batches of $PARALLEL
i=0
batch=1
while [ $i -lt ${#MODELS[@]} ]; do
  batch_end=$((i + PARALLEL))
  [ $batch_end -gt ${#MODELS[@]} ] && batch_end=${#MODELS[@]}
  batch_size=$((batch_end - i))

  echo "========== BATCH $batch: models $((i+1))–${batch_end} of ${#MODELS[@]} =========="

  PIDS=()
  for j in $(seq $i $((batch_end - 1))); do
    IFS=: read -r model needle_id arm_count <<< "${MODELS[$j]}"
    logfile="logs/${model}.log"
    mkdir -p logs
    echo "  LAUNCH $model (→${needle_id}, ${arm_count}-arm) → $logfile"
    bash run_model.sh "$model" "$arm_count" "$needle_id" > "$logfile" 2>&1 &
    PIDS+=($!)
  done

  echo "  waiting for batch $batch (${batch_size} models, PIDs: ${PIDS[*]})..."
  for pid in "${PIDS[@]}"; do
    wait "$pid" 2>/dev/null || true
  done
  echo "  batch $batch complete @ $(date -u +%Y-%m-%dT%H:%M:%SZ)"

  # Progress report
  total_scores=$(find runs/ -name "*.score.json" 2>/dev/null | wc -l | tr -d ' ')
  echo "  total score files: $total_scores"
  echo ""

  i=$batch_end
  batch=$((batch + 1))
done

echo "=== matrix complete @ $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
total=$(find runs/ -name "*.score.json" 2>/dev/null | wc -l | tr -d ' ')
echo "=== total: $total score files ==="

# Consolidate
if [ -f consolidate_scores.py ]; then
  echo ""
  echo "=== consolidating scores ==="
  python3 consolidate_scores.py
fi
