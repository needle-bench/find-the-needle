# needle-bench Scoring

> Version 2.0 — 2026-03-19

## Changes from v1.0

- **Retired** `tokens_per_correct_line` — replaced by `dollars_per_correct_line` (the token-based metric was almost always "Infinity")
- **Added** `estimated_cost_usd`, `dollars_per_correct_line`, `read_tool_ratio`, `tool_calls_per_turn`, `boot_score` / `boot_verdict`, `difficulty_tier`, `autonomy_index`, `effective_cost`
- **Added** composite score `cpu_fitness`
- **Updated** leaderboard ranking: tertiary sort is now `estimated_cost_usd` (replaces `token_cost`)
- **Updated** aggregation: added `weighted_resolve_rate` with difficulty-tier weights

## Overview

Every benchmark run produces 19 metrics. Scoring is deterministic — same run, same scores. Fields that cannot be computed for a given run are `null`, never omitted.

## Metrics

### 1. `resolved` (boolean)

Did the agent's patch make `test.sh` exit 0?

- `true`: test passes after agent modifications
- `false`: test still fails or agent exhausted limits

This is the only metric that matters for the leaderboard headline number. Everything else is texture.

### 2. `turns_to_discovery` (integer)

Number of conversation turns before the agent first identifies the correct file and region containing the bug.

- Measured by comparing agent's edits/mentions against the files touched in `solution.patch`
- If never discovered: set to the turn limit

### 3. `turns_to_fix` (integer)

Number of conversation turns before the agent produces a patch that makes `test.sh` pass.

- If never fixed: set to the turn limit
- `turns_to_fix >= turns_to_discovery` always

### 4. `signal_to_noise` (float, 0.0–1.0)

Ratio of productive actions to total actions.

```
signal_to_noise = productive_turns / total_turns
```

A turn is "productive" if it either:
- Reads/examines a file that is relevant to the bug (touched by solution.patch or its direct dependencies)
- Makes an edit that moves toward the fix

### 5. `false_positives` (integer)

Number of distinct files the agent edited that are NOT touched by `solution.patch`.

- Edits to test files don't count (agents commonly run tests)
- Reverted edits don't count
- Only final-state modifications at scoring time

### 6. `token_cost` (integer)

Total tokens consumed (input + output) across all turns.

- Measured from the model API response metadata
- Includes tool call tokens

### 7. `recovery_events` (integer)

Number of times the agent went down an incorrect path and self-corrected.

A recovery event is detected when:
- Agent reverts a previous edit, OR
- Agent explicitly acknowledges a wrong approach and changes direction

### 8. `recovery_rate` (float, 0.0–1.0)

```
recovery_rate = successful_recoveries / recovery_events
```

A recovery is "successful" if the agent eventually reaches the correct fix after the recovery event. If `recovery_events` is 0, `recovery_rate` is `1.0` (no recovery needed = perfect).

### 9. `wall_clock` (float, seconds)

Total wall-clock time from first agent turn to final scoring.

- Includes all pauses, retries, and tool execution time
- Measured by the harness, not the agent

### 10. `blind_discovery` (boolean)

Did the agent find the bug without the optional `PROMPT` directive?

- `true` if the Agentfile has no `PROMPT` directive and the agent resolved the bug
- `false` otherwise
- Benchmarks with `PROMPT` always score `false` here
- This metric rewards agents that can diagnose from test output alone

### 11. `estimated_cost_usd` (float, dollars)

```
estimated_cost_usd = (input_tokens * input_price_per_M / 1_000_000)
                   + (output_tokens * output_price_per_M / 1_000_000)
```

Dollar cost of the full run using the model's published API pricing. Already computed by runner.py's MetricsRecorder. Cross-vendor comparable.

### 12. `dollars_per_correct_line` (float, dollars)

```
dollars_per_correct_line = estimated_cost_usd / correct_lines_changed
```

- `correct_lines_changed` = lines in the agent's final patch that match lines in `solution.patch`
- If zero correct lines: `null` (not "Infinity")

Replaces the retired `tokens_per_correct_line` from v1.0.

### 13. `read_tool_ratio` (float, 0.0–1.0)

```
read_tool_ratio = file_read_calls / (file_read_calls + bash_cat_calls)
```

Measures whether the model uses the dedicated `file:read` tool vs `cat`-via-bash. Higher = more tool-aware. `null` if `file:read` is not available to the agent.

### 14. `tool_calls_per_turn` (float)

```
tool_calls_per_turn = total_tool_calls / total_turns
```

Separates thrashing (many calls, low signal) from plodding (one call per turn). Neither extreme is ideal; the metric provides signal when combined with `signal_to_noise`.

### 15. `boot_score` (integer, 0–9)

Score from the boot battery, promoted into the unified schema. Models without boot data get `null`.

### 16. `boot_verdict` (enum: `pass` | `partial` | `fail` | `null`)

Categorical verdict from the boot battery. `null` for models that were not boot-tested.

### 17. `difficulty_tier` (enum: `easy` | `medium` | `hard`)

The benchmark's difficulty tier. Set per-benchmark in the Agentfile. Enables weighted scoring (hard bugs worth more).

### 18. `autonomy_index` (float, 0.0–1.0)

```
autonomy_index = (total_turns - correction_turns - stall_turns) / total_turns
```

- `correction_turns` = turns containing errors, reverts, or corrections to previous work
- `stall_turns` = turns with no tool calls or repeated identical commands

Measures whether the model needed babysitting. Higher = more autonomous.

### 19. `effective_cost` (float, tokens)

```
effective_cost = token_cost / historical_resolve_rate_for_tier
```

True deployment cost including expected retries. A cheap model that fails 50% of the time costs more than an expensive one that always succeeds. Uses `0.5` for models with no historical data.

## Composite Scores

### `cpu_fitness` (float, 0.0–1.0)

```
cpu_fitness = resolve_rate^2 * autonomy_index * (1 / log2(effective_cost + 1))
```

The single number: should the OS use this model as its CPU? `resolve_rate` is squared to punish reliability gaps — 70% reliability is not 70% useful for an OS kernel.

## Score Record Format

```json
{
  "benchmark": "off-by-one-redis",
  "agent": "claude-opus-4-20250514",
  "timestamp": "2026-03-19T12:00:00Z",
  "difficulty_tier": "medium",
  "resolved": true,
  "turns_to_discovery": 3,
  "turns_to_fix": 7,
  "signal_to_noise": 0.82,
  "false_positives": 1,
  "token_cost": 45200,
  "estimated_cost_usd": 0.87,
  "dollars_per_correct_line": 0.145,
  "recovery_events": 1,
  "recovery_rate": 1.0,
  "wall_clock": 142.5,
  "blind_discovery": true,
  "read_tool_ratio": 0.91,
  "tool_calls_per_turn": 2.4,
  "boot_score": 7,
  "boot_verdict": "pass",
  "autonomy_index": 0.85,
  "effective_cost": 52941
}
```

## Leaderboard Ranking

Primary sort: `resolved` (descending — solvers first)
Secondary sort: `turns_to_fix` (ascending — fewer turns wins)
Tertiary sort: `estimated_cost_usd` (ascending — cheaper wins)
Quaternary sort: `wall_clock` (ascending)

## Aggregation

When an agent runs multiple benchmarks, aggregate scores are:

- `resolve_rate`: percentage of benchmarks resolved
- `weighted_resolve_rate`: difficulty-weighted resolve rate
  ```
  weighted_resolve_rate = sum(weight_i * resolved_i) / sum(weight_i)
  ```
  Weights: `easy` = 1.0, `medium` = 1.5, `hard` = 2.5
- `mean_turns_to_fix`: geometric mean of turns_to_fix across resolved benchmarks
- `mean_estimated_cost_usd`: geometric mean of estimated_cost_usd across resolved benchmarks
- `blind_discovery_rate`: percentage of PROMPT-free benchmarks resolved
- `mean_autonomy_index`: arithmetic mean of autonomy_index across all runs
- `cpu_fitness`: computed from aggregate resolve_rate, mean autonomy_index, and mean effective_cost
