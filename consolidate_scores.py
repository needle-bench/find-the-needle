#!/usr/bin/env python3
"""
consolidate_scores.py — Aggregate score files into public JSON for the website.

Walks runs/ to find all *-native/, *-kernel/, *-kernel-cpu/ directories,
loads score files, and produces:

  public/scores.json          — flat list of all scores (best per model+bench+arm)
  public/experiment-scores.json — three-arm comparison (native/kernel/kernel-cpu)
"""

import argparse
import json
import os
import re
import sys
from pathlib import Path

RUNS_DIR = Path(__file__).parent / "runs"
SCORES_OUTPUT = Path(__file__).parent / "public" / "scores.json"
EXPERIMENT_OUTPUT = Path(__file__).parent / "public" / "experiment-scores.json"

# Three experiment arms
ARM_PATTERN = re.compile(r"^(.+)-(native|kernel-cpu|kernel)$")

# Rate card for cost computation ($/M tokens: input, output)
RATE_CARD = {
    "claude-haiku-4.5": (1.00, 5.00), "claude-haiku-4-5": (1.00, 5.00),
    "claude-sonnet-4-6": (3.00, 15.00),
    "claude-opus-4-6": (5.00, 25.00), "claude-opus-4.6": (5.00, 25.00),
    "gemini-2.5-flash": (0.30, 2.50), "gemini-2.5-pro": (1.25, 10.00),
    "gemini-3.1-pro-preview": (2.00, 12.00), "gemini-3-flash-preview": (0.50, 3.00),
    "gpt-4.1": (2.00, 8.00), "gpt-5-codex": (1.25, 10.00),
    "o3": (2.00, 8.00), "o4-mini": (1.10, 4.40),
    "grok-3": (3.00, 15.00), "grok-3-fast": (0.20, 0.50), "grok-4": (3.00, 15.00),
    "grok-3-mini": (0.30, 0.50), "grok-4-fast": (0.20, 0.50),
    "grok-4.1-fast": (0.20, 0.50), "grok-4.20": (2.00, 6.00),
    "grok-code-fast-1": (0.20, 1.50),
    "devstral-small-latest": (0.10, 0.30), "devstral-small": (0.10, 0.30),
    "devstral-medium": (0.40, 2.00), "devstral-2512": (0.40, 2.00),
    "codestral-2508": (0.30, 0.90),
    "mistral-small-4-0-26-03": (0.10, 0.30), "mistral-small-119b-2603": (0.10, 0.30),
    "kimi-k2.5": (0.40, 1.99),
    "deepseek-v3.2": (0.26, 0.38), "deepseek-r1": (0.70, 2.50),
    "deepseek-r1-0528": (0.45, 2.15),
    "qwen3-coder-plus": (0.65, 3.25), "qwen3-coder": (0.22, 1.00),
    "qwen3-coder-flash": (0.20, 0.97),
    "llama-4-maverick": (0.15, 0.60),
}


def normalize_model(name: str) -> str:
    """Normalize model name: strip vendor prefix, lowercase, dots→hyphens."""
    name = name.split("/", 1)[1] if "/" in name else name
    return re.sub(r"[_.]", "-", name.lower())


def compute_cost(entry: dict, model: str) -> float:
    """Compute cost from rate card if not already set."""
    existing = entry.get("estimated_cost_usd", 0) or 0
    if existing > 0:
        return existing
    rate = RATE_CARD.get(model)
    if not rate:
        return 0.0
    tin = entry.get("input_tokens", 0) or 0
    tout = entry.get("output_tokens", 0) or 0
    if tin == 0 and tout == 0:
        return 0.0
    return (tin * rate[0] + tout * rate[1]) / 1_000_000


def load_score(fpath: Path) -> dict | None:
    """Load and validate a score JSON file."""
    try:
        with open(fpath) as f:
            data = json.load(f)
    except (json.JSONDecodeError, OSError) as e:
        print(f"  WARN: skipping {fpath} — {e}", file=sys.stderr)
        return None
    if not isinstance(data, dict) or "benchmark" not in data:
        return None
    return data


def arm_summary(entry: dict, model: str) -> dict:
    """Extract per-arm summary fields from a score entry."""
    cost = compute_cost(entry, model)
    return {
        "resolved": bool(entry.get("resolved", False)),
        "turns": entry.get("turns_to_fix", 0) or 0,
        "input_tokens": entry.get("input_tokens", 0) or 0,
        "output_tokens": entry.get("output_tokens", 0) or 0,
        "token_cost": entry.get("token_cost", 0) or 0,
        "cost_usd": round(cost, 6),
        "tool_uses": entry.get("tool_uses", 0) or 0,
        "wall_clock": entry.get("wall_clock", 0) or 0,
        "summary": entry.get("summary", ""),
        "stop_reason": entry.get("stop_reason", ""),
    }


def consolidate_all(dry_run: bool = False) -> None:
    """Main consolidation: walk runs/, produce scores.json + experiment-scores.json."""
    if not RUNS_DIR.exists():
        print(f"ERROR: {RUNS_DIR} not found", file=sys.stderr)
        sys.exit(1)

    # Discover arm directories
    arm_dirs: list[tuple[str, str, Path]] = []  # (model, arm, path)
    for entry in sorted(RUNS_DIR.iterdir()):
        if not entry.is_dir():
            continue
        m = ARM_PATTERN.match(entry.name)
        if m:
            model = m.group(1)
            arm = m.group(2)
            arm_dirs.append((model, arm, entry))

    print(f"Found {len(arm_dirs)} arm directories in {RUNS_DIR}")

    # Load all scores
    all_scores = []  # flat list for scores.json
    # Keyed by (normalized_model, benchmark) → {model, benchmark, native, kernel, cpu}
    experiments: dict[tuple[str, str], dict] = {}

    total_files = 0
    for model, arm, dir_path in arm_dirs:
        for fname in sorted(os.listdir(dir_path)):
            if not fname.endswith(".score.json"):
                continue
            fpath = dir_path / fname
            entry = load_score(fpath)
            if entry is None:
                continue
            total_files += 1

            benchmark = entry["benchmark"]
            entry["computed_cost_usd"] = compute_cost(entry, model)

            # Flat scores list
            all_scores.append(entry)

            # Experiment grouping
            norm = normalize_model(model)
            ek = (norm, benchmark)
            if ek not in experiments:
                experiments[ek] = {
                    "model": model,
                    "model_normalized": norm,
                    "benchmark": benchmark,
                    "native": None,
                    "kernel": None,
                    "cpu": None,
                }

            arm_key = "cpu" if arm == "kernel-cpu" else arm
            current = experiments[ek][arm_key]
            summary = arm_summary(entry, model)

            # Keep best: resolved > not, then fewer turns
            if current is None:
                experiments[ek][arm_key] = summary
            elif summary["resolved"] and not current["resolved"]:
                experiments[ek][arm_key] = summary
            elif summary["resolved"] == current["resolved"] and summary["turns"] < current["turns"]:
                experiments[ek][arm_key] = summary

    # Build experiment output with model aggregates
    exp_list = sorted(experiments.values(), key=lambda e: (e["model"], e["benchmark"]))

    # Compute per-model aggregates for the experiment data
    model_agg: dict[str, dict] = {}
    for exp in exp_list:
        norm = exp["model_normalized"]
        if norm not in model_agg:
            model_agg[norm] = {
                "model": exp["model"],
                "model_normalized": norm,
                "benchmarks": 0,
                "native": {"solved": 0, "total": 0, "cost": 0, "tokens": 0, "tools": 0, "turns": 0, "wall": 0},
                "kernel": {"solved": 0, "total": 0, "cost": 0, "tokens": 0, "tools": 0, "turns": 0, "wall": 0},
                "cpu": {"solved": 0, "total": 0, "cost": 0, "tokens": 0, "tools": 0, "turns": 0, "wall": 0},
            }
        model_agg[norm]["benchmarks"] += 1
        for arm_key in ["native", "kernel", "cpu"]:
            arm_data = exp[arm_key]
            if arm_data is not None:
                agg = model_agg[norm][arm_key]
                agg["total"] += 1
                if arm_data["resolved"]:
                    agg["solved"] += 1
                agg["cost"] += arm_data["cost_usd"]
                agg["tokens"] += arm_data["input_tokens"] + arm_data["output_tokens"]
                agg["tools"] += arm_data["tool_uses"]
                agg["turns"] += arm_data["turns"]
                agg["wall"] += arm_data["wall_clock"]

    # Add rate card info
    for norm, agg in model_agg.items():
        rate = RATE_CARD.get(agg["model"])
        if rate:
            agg["price_per_m_input"] = rate[0]
            agg["price_per_m_output"] = rate[1]

    agg_list = sorted(model_agg.values(), key=lambda a: (
        -max(a["native"]["solved"]/max(a["native"]["total"],1),
             a["kernel"]["solved"]/max(a["kernel"]["total"],1),
             a["cpu"]["solved"]/max(a["cpu"]["total"],1)),
        a["model"]
    ))

    # Summary
    models = sorted(set(e["model_normalized"] for e in exp_list))
    benchmarks = sorted(set(e["benchmark"] for e in exp_list))

    print(f"Loaded {total_files} score files")
    print(f"Models: {len(models)}")
    print(f"Benchmarks: {len(benchmarks)}")
    print(f"Experiment entries: {len(exp_list)}")
    print(f"Model aggregates: {len(agg_list)}")

    # Grand totals
    grand_solved = sum(1 for s in all_scores if s.get("resolved"))
    grand_cost = sum(s.get("computed_cost_usd", 0) for s in all_scores)
    grand_tokens = sum((s.get("input_tokens", 0) or 0) + (s.get("output_tokens", 0) or 0) for s in all_scores)
    print(f"\nGrand totals: {grand_solved}/{len(all_scores)} solved, "
          f"{grand_tokens:,} tokens, ${grand_cost:,.2f} estimated cost")

    if dry_run:
        print(f"\n[dry-run] Would write {len(all_scores)} scores + {len(exp_list)} experiments + {len(agg_list)} aggregates")
        return

    # Write scores.json
    SCORES_OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    with open(SCORES_OUTPUT, "w") as f:
        json.dump(all_scores, f, indent=2)
        f.write("\n")
    print(f"\nWrote {len(all_scores)} entries to {SCORES_OUTPUT}")

    # Write experiment-scores.json with both per-benchmark and aggregates
    experiment_output = {
        "generated": __import__("datetime").datetime.utcnow().isoformat() + "Z",
        "models": agg_list,
        "benchmarks": exp_list,
    }
    with open(EXPERIMENT_OUTPUT, "w") as f:
        json.dump(experiment_output, f, indent=2)
        f.write("\n")
    print(f"Wrote {len(exp_list)} experiments + {len(agg_list)} model aggregates to {EXPERIMENT_OUTPUT}")


def main():
    parser = argparse.ArgumentParser(description="Consolidate needle-bench scores for the website.")
    parser.add_argument("--dry-run", action="store_true", help="Print summary without writing")
    args = parser.parse_args()
    consolidate_all(dry_run=args.dry_run)


if __name__ == "__main__":
    main()
