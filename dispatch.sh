#!/usr/bin/env bash
# dispatch.sh — run needle-bench across models and benchmarks
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BENCHMARKS_DIR="$SCRIPT_DIR/benchmarks"
RUNS_DIR="$SCRIPT_DIR/runs"
MODELS_CONF="$SCRIPT_DIR/models.conf"
MAX_PARALLEL=4
RETRY=0

usage() {
    echo "Usage: dispatch.sh MODEL [--retry]"
    echo "       dispatch.sh --all [--retry]"
    echo ""
    echo "  MODEL     Run all benchmarks for one model"
    echo "  --all     Run all models from models.conf"
    echo "  --retry   Force re-run even if results exist"
    exit 1
}

[[ $# -lt 1 ]] && usage

# Parse args
ALL=0
MODEL=""
for arg in "$@"; do
    case "$arg" in
        --all)    ALL=1 ;;
        --retry)  RETRY=1 ;;
        --help|-h) usage ;;
        *)        MODEL="$arg" ;;
    esac
done

# Discover benchmarks
BENCHMARKS=()
for dir in "$BENCHMARKS_DIR"/*/; do
    name=$(basename "$dir")
    [[ "$name" == "_template" ]] && continue
    BENCHMARKS+=("$name")
done

# Build model list
MODELS=()
if [[ $ALL -eq 1 ]]; then
    while read -r model provider rest; do
        [[ -z "$model" || "$model" == "#"* ]] && continue
        MODELS+=("$model:$provider")
    done < "$MODELS_CONF"
elif [[ -n "$MODEL" ]]; then
    provider=$(grep "^$MODEL " "$MODELS_CONF" 2>/dev/null | awk '{print $2}')
    provider="${provider:-anthropic}"
    MODELS+=("$MODEL:$provider")
else
    usage
fi

# Run benchmarks with parallelism
running=0
for mp in "${MODELS[@]}"; do
    model="${mp%%:*}"
    provider="${mp##*:}"
    for bench in "${BENCHMARKS[@]}"; do
        log="$RUNS_DIR/$model/$bench.jsonl"
        if [[ $RETRY -eq 0 && -f "$log" ]]; then
            echo "SKIP $model/$bench (exists)"
            continue
        fi
        echo "RUN  $model/$bench"
        python3 "$SCRIPT_DIR/runner.py" --model "$model" --benchmark "$bench" --provider "$provider" &
        running=$((running + 1))
        if [[ $running -ge $MAX_PARALLEL ]]; then
            wait -n 2>/dev/null || true
            running=$((running - 1))
        fi
    done
done
wait
echo "Done."
