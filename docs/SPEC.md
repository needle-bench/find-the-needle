# needle-bench Specification

> Version 1.0 — 2026-03-08

## Overview

needle-bench is a benchmark suite for evaluating AI coding agents on real debugging tasks. Each benchmark is a self-contained directory containing a broken codebase inside a Docker container, a test that proves the breakage, and a sealed solution the agent never sees.

The agent gets one prompt: **"find the needle."**

## Benchmark Directory Structure

```
benchmarks/<name>/
  Dockerfile          # builds the broken environment
  Agentfile           # agent configuration
  .bench/
    solution.patch    # sealed ground truth (agent cannot access)
  test.sh             # exit 0 = pass, exit 1 = fail
  README.md           # human-readable description of the bug
```

## Dockerfile Requirements

| Constraint | Value |
|-----------|-------|
| Build time | < 5 minutes on 4-core runner |
| Image size | < 500 MB |
| Base image | Any — Alpine preferred |
| Contents | A real bug in real code. Not synthetic. Not contrived. |

The Dockerfile MUST:
- Install all dependencies needed to build and test the project
- Copy in the broken source code
- NOT include the solution patch or any hints toward the fix
- Produce a runnable environment where `test.sh` fails (exit 1)

The Dockerfile MUST NOT:
- Require network access at test time (build time is fine)
- Mount host volumes
- Require GPU or specialized hardware

## Agentfile Format

The Agentfile configures the agent's environment and constraints.

### Per-benchmark Agentfile

Each benchmark directory may contain an `Agentfile` with static directives:

```
FROM <image>           # Docker image (built from Dockerfile)
PROMPT <text>          # Optional — additional context for the agent
TOOL <tool_name>       # Tools available to the agent (repeatable)
LIMIT turns <N>        # Maximum conversation turns
LIMIT tokens <N>       # Maximum token budget
LIMIT wall_clock <Ns>  # Maximum wall-clock seconds
```

### Unified Agentfile.bench

For `haystack bench` runs, a single `Agentfile.bench` at the project root replaces per-benchmark Agentfiles. It defines tools and default limits; the actual limits are resolved from `difficulty.json` (see below).

```
FROM ${MODEL:-claude-sonnet-4-6}

TOOL shell
TOOL file:read
TOOL file:edit

LIMIT turns ${TURNS:-30}
LIMIT tokens ${TOKENS:-150000}
LIMIT wall_clock ${WALL:-600}
```

### Variable Substitution

Agentfile.bench supports `${VAR:-default}` syntax:

| Variable | Source | Default |
|----------|--------|---------|
| `${MODEL}` | `--model` flag | `claude-sonnet-4-6` |
| `${TURNS}` | difficulty.json tier | `30` |
| `${TOKENS}` | difficulty.json tier | `150000` |
| `${WALL}` | difficulty.json tier | `600` |

The runner resolves limits in this priority order:
1. **difficulty.json tier** — if the benchmark is listed, its tier limits win
2. **Agentfile LIMIT directives** — fallback for unlisted benchmarks
3. **Built-in defaults** — turns=20, tokens=100k, wall=300s

### Difficulty Tiers (difficulty.json)

`difficulty.json` at the project root maps each benchmark to a difficulty tier with resource limits:

| Tier | Benchmarks | Turns | Tokens | Wall Clock |
|------|-----------|-------|--------|------------|
| **Easy** | 8 | 10 | 50,000 | 180s |
| **Medium** | 11 | 30 | 150,000 | 600s |
| **Hard** | 8 | 50 | 300,000 | 900s |

Easy benchmarks are single-cause bugs solvable in under 10 turns. Medium benchmarks involve concurrency, security, or caching bugs. Hard benchmarks require deep domain knowledge (distributed systems, compilers, kernels).

### Directives

| Directive | Required | Description |
|-----------|----------|-------------|
| `FROM` | Yes | Base image reference (supports `${MODEL}` substitution) |
| `PROMPT` | No | Additional context beyond "find the needle" |
| `TOOL` | Yes (1+) | Tools the agent may use (shell, file:read, file:edit) |
| `LIMIT` | Yes (1+) | Resource constraints (supports `${VAR:-default}` substitution) |
| `BOOT` | No | Command to run in container before agent starts |

### Default limits (if not specified)

- `turns`: 30
- `tokens`: 150,000
- `wall_clock`: 600s

## .bench/solution.patch

- Standard unified diff format (`diff -u` or `git diff`)
- Applies cleanly to the broken codebase inside the container
- After applying, `test.sh` MUST exit 0
- This file is NEVER visible to the agent during a run
- Used only for scoring after the run completes

## test.sh

- MUST be executable (`chmod +x`)
- MUST exit 0 on success, exit 1 on failure
- MUST complete in < 60 seconds
- SHOULD produce human-readable output on failure
- MUST NOT require network access
- MUST be deterministic — same input, same result

## Agent Protocol

1. Agent is placed inside the container
2. Agent receives the prompt: "find the needle" (plus optional PROMPT directive)
3. Agent has access to tools listed in the Agentfile
4. Agent explores, diagnoses, and patches the code
5. `test.sh` is run against the agent's modifications
6. Scoring metrics are computed (see SCORING.md)

## Benchmark Naming

- Lowercase, hyphenated: `off-by-one-redis`, `null-deref-tokio`, `race-condition-sqlite`
- Format: `<bug-type>-<project-or-domain>`
- Must be unique across the entire benchmark suite

## Versioning

Each benchmark is immutable once merged. If a benchmark needs modification, a new version is created as a separate directory (e.g., `off-by-one-redis-v2`). The original is never altered.
