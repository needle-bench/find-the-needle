#!/usr/bin/env python3
"""Two-arm experimental runner for needle-bench.

Measures the value of the invisible OS. One prompt. One variable.

Both arms receive: "find the needle."
- bare:   raw container, bash, generic system prompt
- silent: same container, same bash, but silent context silently injected
          into system prompt (boot context, project structure, known issues)

The model can't tell which arm it's in. The delta = the value of ostk.

Usage:
    python3 run_arms.py --model claude-sonnet-4-6 --benchmark off-by-one-pagination
    python3 run_arms.py --model claude-sonnet-4-6 --benchmark off-by-one-pagination --arm bare
    python3 run_arms.py --model claude-sonnet-4-6 --all-benchmarks
"""

import argparse
import json
import os
import subprocess
import sys
import time

from runner import (
    docker_exec, resolve_tools, call_model, detect_provider, do_edit,
    PostRecorder, MetricsRecorder, parse_agentfile,
    _canonical_agent_name, _load_difficulty_json, _model_cost_usd,
    ALL_TOOLS, DEFAULT_TOOLS, SYSTEM_PROMPT,
    list_benchmarks, solution_files,
)

# ── Arm definitions ──────────────────────────────────────────────────────

# One prompt for both arms. The model can't tell which arm it's in.
USER_PROMPT = "find the needle."

# Generic system prompt — no project-specific context
BARE_SYSTEM = "You are a software engineer. You have bash access to a system."

ARM_DEFINITIONS = {
    "bare": {
        "tools": [ALL_TOOLS["bash"]],
        "system_prompt": BARE_SYSTEM,
        "inject_boot_context": False,
        "description": "raw bash, no OS context",
    },
    "silent": {
        "tools": [ALL_TOOLS["bash"]],
        "system_prompt": BARE_SYSTEM,  # same base — silent context appended silently
        "inject_boot_context": True,   # boot.md + project structure injected into system prompt
        "description": "same bash, invisible OS underneath",
    },
}

ALL_ARMS = ["bare", "silent"]


# ── Per-arm benchmark runner ─────────────────────────────────────────────

def run_arm(model, bench_name, bench_dir, provider, arm_name):
    """Run a single arm for a single benchmark. Returns a score dict."""

    canon_model = _canonical_agent_name(model)
    start_time = time.time()

    try:
        return _run_arm_inner(model, bench_name, bench_dir, provider, arm_name)
    except Exception as e:
        # Write a FAIL score so every benchmark produces a .score.json
        error_score = {
            "benchmark": bench_name,
            "agent": f"{canon_model}-{arm_name}",
            "arm": arm_name,
            "control_type": arm_name,
            "resolved": False,
            "error": str(e),
            "turns_to_fix": 0,
            "token_cost": 0,
            "estimated_cost_usd": 0,
            "wall_clock": round(time.time() - start_time, 1),
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        }
        proj_root = os.path.dirname(os.path.abspath(__file__))
        runs_dir = os.path.join(proj_root, "runs", f"{canon_model}-{arm_name}")
        os.makedirs(runs_dir, exist_ok=True)
        score_path = os.path.join(runs_dir, f"{bench_name}.score.json")
        with open(score_path, "w") as f:
            json.dump(error_score, f, indent=2)
        run_score_dir = os.path.join(runs_dir, bench_name)
        os.makedirs(run_score_dir, exist_ok=True)
        run_score_path = os.path.join(run_score_dir, "score.json")
        with open(run_score_path, "w") as f:
            json.dump(error_score, f, indent=2)
        print(f"  [{arm_name}] FAIL: {e}", file=sys.stderr)
        return error_score


def _run_arm_inner(model, bench_name, bench_dir, provider, arm_name):
    """Inner implementation of run_arm — may raise exceptions."""

    arm = ARM_DEFINITIONS[arm_name]
    api_model = model
    canon_model = _canonical_agent_name(model)
    agent_name = f"{canon_model}-{arm_name}"

    # Parse benchmark Agentfile (for BOOT, limits, etc.)
    proj_root = os.path.dirname(os.path.abspath(__file__))
    bench_agentfile = os.path.join(proj_root, "Agentfile.bench")
    per_bench_agentfile = os.path.join(bench_dir, "Agentfile")
    if os.path.exists(bench_agentfile):
        cfg = parse_agentfile(bench_agentfile)
    else:
        cfg = parse_agentfile(per_bench_agentfile)

    # Also read per-benchmark Agentfile for BOOT directive
    bench_cfg = parse_agentfile(per_bench_agentfile) if os.path.exists(per_bench_agentfile) else cfg

    sol_files = solution_files(bench_dir)

    # Resolve limits: difficulty.json tier > Agentfile LIMIT > defaults
    tiers, benchmarks_map = _load_difficulty_json()
    diff_limits = None
    if tiers and benchmarks_map:
        tier_name = benchmarks_map.get(bench_name)
        if tier_name:
            diff_limits = tiers.get(tier_name)

    if diff_limits is not None:
        max_turns = diff_limits.get("turns", 30)
        max_tokens = diff_limits.get("tokens", 150000)
        max_wall = diff_limits.get("wall_clock", 600)
    else:
        limits = cfg["limits"]
        max_turns = limits.get("turns", 20)
        max_tokens = limits.get("tokens", 100000)
        max_wall = limits.get("wall_clock", 300)

    # Difficulty tier for score record
    difficulty_tier = (benchmarks_map or {}).get(bench_name, "medium") if benchmarks_map else "medium"

    # Output paths: runs/{model}-{arm}/{benchmark}.score.json
    runs_dir = os.path.join(proj_root, "runs", agent_name)
    os.makedirs(runs_dir, exist_ok=True)
    log_path = os.path.join(runs_dir, f"{bench_name}.jsonl")

    post = PostRecorder(runs_dir, bench_name, agent_name)
    metrics = MetricsRecorder(runs_dir, bench_name, agent_name)

    # Build Docker image
    print(f"  [{arm_name}] Building Docker image...", file=sys.stderr)
    subprocess.run(
        ["docker", "build", "-t", f"needle-bench-{bench_name}", bench_dir],
        capture_output=True, check=True,
    )

    # Start container — name includes arm to avoid collisions
    # Container auto-dies after max_wall + 60s buffer (prevents zombies)
    ts = str(int(time.time()))
    container = f"nb-{canon_model.replace('/', '-')}-{arm_name}-{bench_name}-{ts}"
    container_timeout = max_wall + 60
    subprocess.run(
        ["docker", "run", "-d", "--name", container,
         f"needle-bench-{bench_name}", "sleep", str(container_timeout)],
        capture_output=True, check=True,
    )

    # Detect WORKDIR from the running container
    _, wdir_out, _ = docker_exec(container, "pwd")
    workdir = wdir_out.strip() or "/workspace"

    # Snapshot workspace before agent starts
    docker_exec(container, f"cp -a {workdir} {workdir}.orig")
    docker_exec(container, f"cd {workdir} && git init -q && git add -A && git commit -q -m baseline")

    # ── Same tools, same prompt for both arms ───────────────────────
    tools = arm["tools"]
    system_prompt = arm["system_prompt"]

    # Silent arm: silently inject silent context into system prompt
    # The model doesn't know this context came from ostk — it just has better info
    if arm.get("inject_boot_context"):
        # Read project structure
        _, ls_out, _ = docker_exec(container, f"find {workdir} -type f -name '*.py' -o -name '*.go' -o -name '*.rs' -o -name '*.java' -o -name '*.js' -o -name '*.ts' -o -name '*.c' -o -name '*.sh' 2>/dev/null | head -30")
        # Read test output
        _, test_out, _ = docker_exec(container, "bash test.sh 2>&1 | head -20")
        # Read any README
        _, readme_out, _ = docker_exec(container, f"cat {workdir}/README.md 2>/dev/null | head -30")

        silent_context = ""
        if readme_out.strip():
            silent_context += f"Project overview:\n{readme_out.strip()}\n\n"
        if ls_out.strip():
            silent_context += f"Source files:\n{ls_out.strip()}\n\n"
        if test_out.strip():
            silent_context += f"Current test output (test.sh):\n{test_out.strip()}\n\n"

        if silent_context:
            system_prompt = system_prompt + "\n\n" + silent_context

    # Same user message for both arms — the model can't tell which arm it's in
    instance_prompt = USER_PROMPT

    # POST start: capture initial test output
    _irc, _istdout, _istderr = docker_exec(container, "bash test.sh")
    initial_test_output = _istdout + ("\n" + _istderr if _istderr else "")
    post.start(initial_test_output, instance_prompt)

    log_f = open(log_path, "w")
    start_time = time.time()
    total_tokens_in = 0
    total_tokens_out = 0
    total_cost_usd = 0.0
    turn_events = []
    total_tool_calls = 0
    total_read_calls = 0
    total_cat_calls = 0

    def emit(event):
        log_f.write(json.dumps(event) + "\n")
        log_f.flush()

    emit({
        "event": "run.start",
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "benchmark": bench_name, "model": canon_model, "arm": arm_name,
        "tools": [t["name"] for t in tools],
        "system_prompt_preview": system_prompt[:200],
    })

    messages = [{"role": "user", "content": instance_prompt}]
    final_test_exit = 1
    final_test_output = ""

    try:
        for turn in range(1, max_turns + 1):
            elapsed = time.time() - start_time
            if elapsed >= max_wall:
                break
            if total_tokens_in + total_tokens_out >= max_tokens:
                break

            resp = call_model(api_model, messages, provider,
                              system_prompt=system_prompt, tools=tools)
            tokens_in = resp.get("usage", {}).get("input_tokens", 0)
            tokens_out = resp.get("usage", {}).get("output_tokens", 0)
            turn_cost_usd = resp.get("usage", {}).get("cost_usd", 0)
            total_tokens_in += tokens_in
            total_tokens_out += tokens_out
            total_cost_usd += turn_cost_usd

            metrics.record_turn(turn, tokens_in, tokens_out)

            content_blocks = resp.get("content", [])
            stop_reason = resp.get("stop_reason", "end_turn")

            files_edited = []
            files_read = []
            test_exit = None

            messages.append({"role": "assistant", "content": content_blocks})

            tool_results = []
            for block in content_blocks:
                if block.get("type") != "tool_use":
                    continue
                name = block["name"]
                inp = block.get("input", {})
                tool_id = block.get("id", "")
                total_tool_calls += 1

                if name == "bash":
                    cmd = inp.get("command", "")
                    if cmd.strip().startswith("cat "):
                        total_cat_calls += 1
                    for token in cmd.split():
                        if token.startswith("/dev"):
                            continue
                        if token.startswith("/"):
                            files_read.append(token)
                        elif "." in token and not token.startswith("-"):
                            files_read.append(workdir + "/" + token)
                    rc, stdout, stderr = docker_exec(container, cmd)
                    output = stdout
                    if stderr:
                        output += ("\n" if output else "") + stderr
                    tool_results.append({
                        "type": "tool_result", "tool_use_id": tool_id,
                        "content": output if output else f"(exit {rc})",
                    })
                    post.bash(cmd, output, turn)

                    # For bare/bare arms: detect if the model ran test.sh via bash
                    if "test.sh" in cmd:
                        test_exit = rc
                        final_test_output = output

                elif name == "read":
                    total_read_calls += 1
                    path = inp.get("path", "")
                    files_read.append(path)
                    rc, stdout, stderr = docker_exec(container, f"cat {path!r}")
                    output = stdout
                    if rc != 0:
                        output = f"ERROR: {stderr}" if stderr else f"(exit {rc})"
                    tool_results.append({
                        "type": "tool_result", "tool_use_id": tool_id,
                        "content": output if output else "(empty file)",
                    })
                    post.read(path, turn)

                elif name == "edit":
                    path = inp.get("path", "")
                    old_str = inp.get("old_str", "")
                    new_str = inp.get("new_str", "")
                    rc, stdout, stderr = do_edit(container, path, old_str, new_str)
                    if rc == 0:
                        files_edited.append(path)
                    result_text = stdout if rc == 0 else f"ERROR: {stderr}"
                    tool_results.append({
                        "type": "tool_result", "tool_use_id": tool_id,
                        "content": result_text,
                    })
                    post.edit(path, old_str, new_str, turn)

                    # Run test.sh after every successful edit
                    if rc == 0:
                        trc, tstdout, tstderr = docker_exec(container, "bash test.sh")
                        test_exit = trc
                        test_output = tstdout
                        if tstderr:
                            test_output += ("\n" if test_output else "") + tstderr
                        final_test_output = test_output

            turn_event = {
                "event": "turn", "turn": turn,
                "files_edited": files_edited, "files_read": files_read,
                "tokens_in": tokens_in, "tokens_out": tokens_out,
                "test_exit": test_exit,
            }
            emit(turn_event)
            turn_events.append(turn_event)

            if tool_results:
                messages.append({"role": "user", "content": tool_results})

            # Send test output as separate user message
            if final_test_output and test_exit is not None:
                messages.append({
                    "role": "user",
                    "content": f"[test.sh exit={test_exit}]\n{final_test_output}",
                })

            if test_exit == 0:
                final_test_exit = 0
                break

            if stop_reason != "tool_use":
                break

    finally:
        # Final test run if not already passed
        if final_test_exit != 0:
            trc, fout, ferr = docker_exec(container, "bash test.sh")
            final_test_exit = trc
            final_test_output = fout + ("\n" + ferr if ferr else "")

        # Count correct lines vs solution.patch
        correct_lines = 0
        patch_path = os.path.join(bench_dir, ".bench", "solution.patch")
        if os.path.exists(patch_path):
            with open(patch_path) as f:
                patch_adds = [l[1:].strip() for l in f if l.startswith("+") and not l.startswith("+++")]
            drc, diff_out, _ = docker_exec(
                container,
                f"cd {workdir} && git diff 2>/dev/null || diff -ruN {workdir}.orig {workdir} 2>/dev/null",
            )
            agent_adds = [l[1:].strip() for l in diff_out.splitlines()
                          if l.startswith("+") and not l.startswith("+++")]
            for line in patch_adds:
                if line and line in agent_adds:
                    correct_lines += 1

        wall_clock = time.time() - start_time

        emit({
            "event": "run.end",
            "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
            "test_exit": final_test_exit, "total_turns": len(turn_events),
            "correct_lines": correct_lines, "arm": arm_name,
        })
        log_f.close()

        resolved_flag = final_test_exit == 0
        post.end(resolved_flag, final_test_output, len(turn_events))
        post.close()

        cum_in, cum_out, total_tok, cost = metrics.complete()
        metrics.close()
        tok_fmt = f"{total_tok:,}"
        print(f"  [{arm_name}] tokens: {tok_fmt} total (${cost:.4f})", file=sys.stderr)

        # Cleanup container
        subprocess.run(["docker", "rm", "-f", container], capture_output=True)

    # ── Compute scores ───────────────────────────────────────────────
    resolved = final_test_exit == 0
    total_turns = len(turn_events)
    token_cost = total_tokens_in + total_tokens_out

    # turns_to_discovery
    turns_to_discovery = max_turns
    for te in turn_events:
        for f_path in te.get("files_edited", []) + te.get("files_read", []):
            basename = f_path.lstrip("/")
            for prefix in ("workspace/", "app/"):
                if basename.startswith(prefix):
                    basename = basename[len(prefix):]
                    break
            if any(basename == sf or f_path.endswith(sf) for sf in sol_files):
                turns_to_discovery = te["turn"]
                break
        if turns_to_discovery != max_turns:
            break

    # turns_to_fix
    turns_to_fix = max_turns
    for te in turn_events:
        if te.get("test_exit") == 0:
            turns_to_fix = te["turn"]
            break

    # signal_to_noise
    productive = 0
    for te in turn_events:
        edited = te.get("files_edited", [])
        read = te.get("files_read", [])
        is_productive = False
        for f_path in edited + read:
            basename = f_path.lstrip("/")
            for prefix in ("workspace/", "app/"):
                if basename.startswith(prefix):
                    basename = basename[len(prefix):]
                    break
            if any(basename == sf or f_path.endswith(sf) for sf in sol_files):
                is_productive = True
                break
        if is_productive:
            productive += 1
    signal_to_noise = productive / total_turns if total_turns > 0 else 0.0

    # false_positives
    all_edited = set()
    for te in turn_events:
        for f_path in te.get("files_edited", []):
            all_edited.add(f_path)
    false_pos = 0
    for f_path in all_edited:
        basename = f_path.lstrip("/")
        for prefix in ("workspace/", "app/"):
            if basename.startswith(prefix):
                basename = basename[len(prefix):]
                break
        if not any(basename == sf or f_path.endswith(sf) for sf in sol_files):
            if "test" not in f_path.lower():
                false_pos += 1

    # tokens_per_correct_line
    tpcl = None if correct_lines == 0 else token_cost / correct_lines

    # estimated_cost_usd — prefer real OpenRouter cost, fall back to MODEL_PRICING
    estimated_cost_usd = total_cost_usd if total_cost_usd > 0 else cost

    # dollars_per_correct_line
    dollars_per_correct_line = round(estimated_cost_usd / correct_lines, 6) if correct_lines > 0 else None

    # tool_calls_per_turn
    tool_calls_per_turn = round(total_tool_calls / max(total_turns, 1), 2)

    # read_tool_ratio
    _total_reads = total_read_calls + total_cat_calls
    read_tool_ratio = round(total_read_calls / _total_reads, 2) if _total_reads > 0 else None

    # Benchmark commit hash
    try:
        _bench_sha = subprocess.check_output(
            ["git", "-C", bench_dir, "rev-parse", "--short", "HEAD"],
            stderr=subprocess.DEVNULL, text=True,
        ).strip()
    except Exception:
        _bench_sha = "unknown"

    score = {
        "benchmark": bench_name,
        "agent": agent_name,
        "arm": arm_name,
        "control_type": arm_name,
        "commit": _bench_sha,
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "difficulty_tier": difficulty_tier,
        "resolved": resolved,
        "turns_to_discovery": turns_to_discovery,
        "turns_to_fix": turns_to_fix,
        "signal_to_noise": round(signal_to_noise, 3),
        "false_positives": false_pos,
        "token_cost": token_cost,
        "estimated_cost_usd": estimated_cost_usd,
        "dollars_per_correct_line": dollars_per_correct_line,
        "tokens_per_correct_line": tpcl,
        "wall_clock": round(wall_clock, 1),
        "tool_calls_per_turn": tool_calls_per_turn,
        "read_tool_ratio": read_tool_ratio,
    }

    # Write score files
    score_path = os.path.join(runs_dir, f"{bench_name}.score.json")
    with open(score_path, "w") as f:
        json.dump(score, f, indent=2)

    run_score_path = os.path.join(runs_dir, bench_name, "score.json")
    os.makedirs(os.path.dirname(run_score_path), exist_ok=True)
    with open(run_score_path, "w") as f:
        json.dump(score, f, indent=2)

    return score


# ── Summary printer ──────────────────────────────────────────────────────

def print_summary(bench_name, model, scores):
    """Print a comparison table for all arms run on a single benchmark."""
    print(f"\n=== {bench_name} ({model}) ===")
    for arm_name in ALL_ARMS:
        s = scores.get(arm_name)
        if s is None:
            continue
        resolved_mark = "\u2713" if s["resolved"] else "\u2717"
        turns = s["turns_to_fix"]
        tok = s["token_cost"]
        cost = s["estimated_cost_usd"]
        wall = s["wall_clock"]

        # Format token count with k suffix
        if tok >= 1000:
            tok_str = f"{tok // 1000}k tok"
        else:
            tok_str = f"{tok} tok"

        print(f"  {arm_name:8s} {resolved_mark}  {turns:2d}t  {tok_str:>8s}  ${cost:.3f}  {wall:.0f}s")

    # Delta: silent vs bare (if both present)
    bare = scores.get("bare")
    silent = scores.get("silent")
    if bare and silent:
        solve_delta = int(silent["resolved"]) - int(bare["resolved"])
        tok_delta = silent["token_cost"] - bare["token_cost"]
        cost_delta = silent["estimated_cost_usd"] - bare["estimated_cost_usd"]
        wall_delta = silent["wall_clock"] - bare["wall_clock"]

        solve_str = f"+{solve_delta}" if solve_delta >= 0 else str(solve_delta)
        tok_delta_str = f"{tok_delta // 1000}k" if abs(tok_delta) >= 1000 else str(tok_delta)
        if tok_delta >= 0:
            tok_delta_str = f"+{tok_delta_str}"

        print(f"\n  Delta (silent vs bare): {solve_str} solves, "
              f"{tok_delta_str} tok, ${cost_delta:+.3f}, {wall_delta:+.0f}s")
    print()


# ── CLI entrypoint ───────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(
        description="Four-arm experimental runner for needle-bench",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=(
            "Arms (same prompt, same tools — one variable):\n"
            "  bare   — raw bash, generic system prompt, no OS context\n"
            "  silent — same bash, but kernel context silently injected into system prompt\n"
            "\n"
            "Both arms receive: 'find the needle.'\n"
            "The delta = the value of the invisible OS.\n"
        ),
    )
    parser.add_argument("--model", required=True,
                        help="Model name (e.g. claude-sonnet-4-6)")
    parser.add_argument("--provider", default="openrouter",
                        help="API provider (default: openrouter)")
    parser.add_argument("--benchmark",
                        help="Specific benchmark name")
    parser.add_argument("--all-benchmarks", action="store_true",
                        help="Run all benchmarks")
    parser.add_argument("--arm", default=None,
                        help="Comma-separated arms to run (default: all four)")
    parser.add_argument("--dry-run", action="store_true",
                        help="Print what would run without executing")
    args = parser.parse_args()

    # Validate provider API key
    if args.provider == "openrouter" and not os.environ.get("OPENROUTER_API_KEY"):
        print("ERROR: OPENROUTER_API_KEY not set", file=sys.stderr)
        sys.exit(1)
    if args.provider == "anthropic" and not os.environ.get("ANTHROPIC_API_KEY"):
        print("ERROR: ANTHROPIC_API_KEY not set", file=sys.stderr)
        sys.exit(1)
    if args.provider == "google" and not os.environ.get("GOOGLE_API_KEY"):
        print("ERROR: GOOGLE_API_KEY not set", file=sys.stderr)
        sys.exit(1)

    # Resolve benchmarks
    if args.all_benchmarks:
        benchmarks = list_benchmarks()
    elif args.benchmark:
        benchmarks = [args.benchmark]
    else:
        print("ERROR: --benchmark or --all-benchmarks required", file=sys.stderr)
        sys.exit(1)

    # Resolve arms
    if args.arm:
        arms = [a.strip() for a in args.arm.split(",")]
        for a in arms:
            if a not in ARM_DEFINITIONS:
                print(f"ERROR: unknown arm '{a}'. Valid: {', '.join(ALL_ARMS)}",
                      file=sys.stderr)
                sys.exit(1)
    else:
        arms = list(ALL_ARMS)

    base = os.path.dirname(os.path.abspath(__file__))
    canon_model = _canonical_agent_name(args.model)

    # Dry run mode
    if args.dry_run:
        print(f"Model:     {args.model} (canonical: {canon_model})")
        print(f"Provider:  {args.provider}")
        print(f"Arms:      {', '.join(arms)}")
        print(f"Benchmarks ({len(benchmarks)}):")
        for b in benchmarks:
            for a in arms:
                agent = f"{canon_model}-{a}"
                container = f"nb-{canon_model.replace('/', '-')}-{a}-{b}-TIMESTAMP"
                out_dir = f"runs/{agent}/"
                print(f"  {a:8s} x {b:40s} -> {out_dir}{b}.score.json")
        print(f"\nTotal runs: {len(benchmarks) * len(arms)}")
        return

    # Run all combinations
    all_results = {}  # bench_name -> {arm_name -> score}

    for bench in benchmarks:
        bench_dir = os.path.join(base, "benchmarks", bench)
        if not os.path.isdir(bench_dir):
            print(f"ERROR: benchmark not found: {bench}", file=sys.stderr)
            continue

        print(f"\n{'=' * 60}", file=sys.stderr)
        print(f"=== {bench} ({args.model}) ===", file=sys.stderr)
        print(f"{'=' * 60}", file=sys.stderr)

        bench_scores = {}
        for arm_name in arms:
            print(f"\n  [{arm_name}] Starting...", file=sys.stderr)
            try:
                score = run_arm(args.model, bench, bench_dir, args.provider, arm_name)
                bench_scores[arm_name] = score
                resolved_mark = "\u2713" if score["resolved"] else "\u2717"
                print(f"  [{arm_name}] {resolved_mark}  {score['turns_to_fix']}t  "
                      f"{score['token_cost']} tok  ${score['estimated_cost_usd']:.3f}  "
                      f"{score['wall_clock']:.0f}s", file=sys.stderr)
            except Exception as e:
                print(f"  [{arm_name}] ERROR: {e}", file=sys.stderr)
                import traceback
                traceback.print_exc(file=sys.stderr)

        all_results[bench] = bench_scores

        # Print summary for this benchmark (to stdout)
        if bench_scores:
            print_summary(bench, args.model, bench_scores)

    # Final aggregate summary if multiple benchmarks
    if len(all_results) > 1:
        print(f"\n{'=' * 60}")
        print(f"AGGREGATE RESULTS ({args.model}, {len(all_results)} benchmarks)")
        print(f"{'=' * 60}")

        for arm_name in arms:
            arm_scores = [v[arm_name] for v in all_results.values() if arm_name in v]
            if not arm_scores:
                continue
            total = len(arm_scores)
            solved = sum(1 for s in arm_scores if s["resolved"])
            avg_turns = sum(s["turns_to_fix"] for s in arm_scores) / total
            avg_tok = sum(s["token_cost"] for s in arm_scores) / total
            avg_cost = sum(s["estimated_cost_usd"] for s in arm_scores) / total
            avg_wall = sum(s["wall_clock"] for s in arm_scores) / total
            print(f"  {arm_name:8s}  {solved}/{total} solved  "
                  f"avg {avg_turns:.1f}t  {avg_tok:.0f} tok  "
                  f"${avg_cost:.3f}  {avg_wall:.0f}s")

        # Aggregate delta: silent vs bare
        bare_scores = [v["bare"] for v in all_results.values() if "bare" in v]
        silent_scores = [v["silent"] for v in all_results.values() if "silent" in v]
        if bare_scores and silent_scores:
            bare_solved = sum(1 for s in bare_scores if s["resolved"])
            silent_solved = sum(1 for s in silent_scores if s["resolved"])
            solve_delta = silent_solved - bare_solved
            avg_tok_delta = (sum(s["token_cost"] for s in silent_scores) / len(silent_scores)
                            - sum(s["token_cost"] for s in bare_scores) / len(bare_scores))
            avg_cost_delta = (sum(s["estimated_cost_usd"] for s in silent_scores) / len(silent_scores)
                             - sum(s["estimated_cost_usd"] for s in bare_scores) / len(bare_scores))
            print(f"\n  Delta (silent vs bare): {solve_delta:+d} solves, "
                  f"{avg_tok_delta:+.0f} avg tok, ${avg_cost_delta:+.3f} avg cost")
        print()


if __name__ == "__main__":
    main()
