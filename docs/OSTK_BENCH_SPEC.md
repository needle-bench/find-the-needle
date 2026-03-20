# ostk bench — kernel-native benchmark runner

> Spec for the ostk kernel team. This describes what `ostk bench <scenario>` needs to do
> to replace `runner.py` in needle-bench.

## Current state
- `ostk bench` is wired to the CLI (src/commands/bench.rs)
- bench.rs delegates Docker scenarios to `runner.py` via subprocess
- runner.py manages its own agent loop, API calls, and tool execution

## Target state
- `ostk bench` runs the full pipeline natively — no Python, no runner.py
- The kernel's agent loop (cpu/agent_loop.rs) drives the model
- MCP tools are routed through `docker exec` when in bench mode

## What needs to change in the kernel

### 1. Docker execution backend
When `ostk bench <scenario>` runs:
1. `docker build -t needle-bench-<scenario> benchmarks/<scenario>/`
2. `docker run -d --name nb-<scenario> needle-bench-<scenario> sleep 3600`
3. Initial setup inside container:
   - `docker exec nb-<scenario> cp -a /workspace /workspace.orig`
   - `docker exec nb-<scenario> bash -c 'cd /workspace && git init && git add -A && git commit -m init'`

### 2. MCP tool routing through Docker
When bench mode is active, MCP tools execute inside the container:
- `shell(cmd)` → `docker exec nb-<scenario> bash -c '<cmd>'`
- `file:read(path)` → `docker exec nb-<scenario> cat <path>`
- `file:edit(path, old, new)` → route through the kernel's CAS edit, but targeting the container filesystem

The agent doesn't know it's in a container. The tools are the same. Only the execution backend changes.

### 3. Agent loop integration
- Read `Agentfile.bench` for tool list
- Read `difficulty.json` for limits (turns, tokens, wall_clock)
- The agent gets a system prompt: "You are debugging a codebase. The tests are failing. Find and fix the bug. Run test.sh to verify."
- The kernel enforces limits (max turns, max tokens, wall clock timeout)
- After each `file:edit`, automatically run `test.sh` and feed result back

### 4. Scoring
After the agent loop completes (resolved or exhausted):
- `resolved` = final `docker exec nb-<scenario> bash -c './test.sh; echo $?'` == 0
- Extract metrics from the agent loop: turns, tokens, wall_clock, tool calls
- Read `benchmarks/<scenario>/.bench/solution.patch` for scoring comparisons
- Write score to `runs/<model>/<scenario>.score.json`

### 5. Cleanup
- `docker stop nb-<scenario> && docker rm nb-<scenario>`
- Optionally: `docker rmi needle-bench-<scenario>` if --cleanup flag

### 6. Compose support (Tier 2)
If `benchmarks/<scenario>/compose.yml` exists:
- `docker compose -f benchmarks/<scenario>/compose.yml up -d`
- Route MCP tools to the primary service container
- `docker compose down` on cleanup

### 7. Model selection
- `--model` flag selects the model (default: claude-sonnet-4-6)
- The kernel already supports multiple models (Anthropic, Google, etc.)
- No need for OpenRouter — the kernel handles model dispatch natively

## What stays in needle-bench
- `Agentfile.bench` — tool configuration
- `difficulty.json` — tier limits
- `consolidate_scores.py` — score aggregation
- `score_boot.py` — boot battery scoring
- `benchmarks/` — the scenarios themselves
- CI workflows — trigger `ostk bench` instead of `python3 runner.py`

## What gets removed from needle-bench
- `runner.py` (866 lines) — replaced by kernel
- `run_needle_bench.py` (280 lines) — alternative runner, also replaced
- `dispatch.sh` (81 lines) — replaced by CI matrix + `ostk bench`
- `models.conf` — model selection moves to CLI flags
- `score_trajectory.py` — scoring moves to kernel
