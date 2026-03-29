#!/usr/bin/env python3
"""Run control arms for needle-bench benchmarks.

Two controls:
1. solution-applied: Apply .bench/solution.patch, run test.sh -> should PASS
2. no-agent: Run test.sh with zero changes -> should FAIL

These prove the benchmarks are valid: the bug is real and the fix works.

Usage:
    python3 run_control.py                    # run all benchmarks
    python3 run_control.py --benchmark off-by-one-pagination  # one benchmark
    python3 run_control.py --dry-run          # show what would run
"""

import argparse
import json
import os
import subprocess
import sys
import time

BASE_DIR = os.path.dirname(os.path.abspath(__file__))
BENCHMARKS_DIR = os.path.join(BASE_DIR, "benchmarks")
RUNS_DIR = os.path.join(BASE_DIR, "runs", "control")

# Benchmarks to skip (not real code-solve benchmarks)
SKIP = {"_template", "haystack-boot", "haystack-mint"}


def list_benchmarks():
    """Return sorted list of benchmark names, excluding skipped ones."""
    names = []
    for name in sorted(os.listdir(BENCHMARKS_DIR)):
        if name in SKIP or name.startswith("_"):
            continue
        bench_dir = os.path.join(BENCHMARKS_DIR, name)
        if os.path.isdir(bench_dir):
            names.append(name)
    return names


def docker_build(bench_name, bench_dir):
    """Build the Docker image for a benchmark. Returns True on success."""
    tag = f"nb-ctrl-{bench_name}"
    result = subprocess.run(
        ["docker", "build", "-t", tag, bench_dir],
        capture_output=True, text=True, timeout=300,
    )
    if result.returncode != 0:
        print(f"  BUILD FAILED: {result.stderr[-500:]}", file=sys.stderr)
        return False
    return True


def docker_run_container(bench_name):
    """Start a fresh container. Returns container name."""
    tag = f"nb-ctrl-{bench_name}"
    ts = str(int(time.time()))
    container = f"nb-ctrl-{bench_name}-{ts}"
    subprocess.run(
        ["docker", "run", "-d", "--name", container, tag, "sleep", "3600"],
        capture_output=True, text=True, check=True, timeout=30,
    )
    return container


def docker_exec(container, cmd, timeout=120):
    """Execute a command inside the container. Returns (returncode, stdout, stderr)."""
    result = subprocess.run(
        ["docker", "exec", container, "bash", "-c", cmd],
        capture_output=True, text=True, timeout=timeout,
    )
    stdout = result.stdout[-4000:] if len(result.stdout) > 4000 else result.stdout
    stderr = result.stderr[-2000:] if len(result.stderr) > 2000 else result.stderr
    return result.returncode, stdout, stderr


def docker_rm(container):
    """Remove a container."""
    subprocess.run(
        ["docker", "rm", "-f", container],
        capture_output=True, text=True,
    )


def detect_workdir(bench_dir):
    """Detect the final WORKDIR from a Dockerfile.

    Reads the Dockerfile and returns the last WORKDIR directive.
    Falls back to /workspace if none found.
    """
    dockerfile = os.path.join(bench_dir, "Dockerfile")
    workdir = "/workspace"
    try:
        with open(dockerfile) as f:
            for line in f:
                stripped = line.strip()
                if stripped.upper().startswith("WORKDIR"):
                    parts = stripped.split(None, 1)
                    if len(parts) == 2:
                        workdir = parts[1]
    except FileNotFoundError:
        pass
    return workdir


def run_test(container, workdir):
    """Run test.sh in the container's workdir. Returns (passed: bool, wall_clock: float)."""
    t0 = time.time()
    rc, stdout, stderr = docker_exec(container, f"cd {workdir} && bash test.sh")
    elapsed = time.time() - t0
    return rc == 0, elapsed


def write_score(bench_name, agent, resolved, wall_clock, control_type):
    """Write a control score JSON file."""
    os.makedirs(RUNS_DIR, exist_ok=True)
    score = {
        "benchmark": bench_name,
        "agent": agent,
        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "resolved": resolved,
        "turns_to_fix": 0,
        "token_cost": 0,
        "wall_clock": round(wall_clock, 1),
        "control_type": control_type,
    }
    # Use agent-prefixed filename to avoid collisions between the two controls
    score_path = os.path.join(RUNS_DIR, f"{bench_name}.{agent}.score.json")
    with open(score_path, "w") as f:
        json.dump(score, f, indent=2)
        f.write("\n")
    return score


def run_no_agent(bench_name, bench_dir):
    """No-agent control: run test.sh with zero modifications.

    Expected: test FAILS (resolved=False). If it passes, the benchmark is broken.
    """
    workdir = detect_workdir(bench_dir)
    container = docker_run_container(bench_name)
    try:
        passed, elapsed = run_test(container, workdir)
        score = write_score(
            bench_name,
            agent="control-baseline",
            resolved=passed,
            wall_clock=elapsed,
            control_type="no-agent",
        )
        return score
    finally:
        docker_rm(container)


def run_solution_applied(bench_name, bench_dir):
    """Solution-applied control: apply .bench/solution.patch, then run test.sh.

    Expected: test PASSES (resolved=True). If it fails, the solution patch is broken.
    """
    patch_path = os.path.join(bench_dir, ".bench", "solution.patch")
    if not os.path.exists(patch_path):
        print(f"  SKIP solution-applied: no .bench/solution.patch", file=sys.stderr)
        return None

    workdir = detect_workdir(bench_dir)
    container = docker_run_container(bench_name)
    try:
        # Copy the solution patch into the container
        subprocess.run(
            ["docker", "cp", patch_path, f"{container}:/tmp/solution.patch"],
            capture_output=True, text=True, check=True, timeout=30,
        )

        # Apply the patch
        rc, stdout, stderr = docker_exec(
            container, f"cd {workdir} && git apply /tmp/solution.patch"
        )
        if rc != 0:
            # Some benchmarks may not have git initialized; try plain patch
            rc2, stdout2, stderr2 = docker_exec(
                container, f"cd {workdir} && patch -p1 < /tmp/solution.patch"
            )
            print(f"DEBUG: git apply rc={rc} stderr={stderr.strip()} stdout={stdout.strip()}")
            print(f"DEBUG: patch rc={rc2} stderr={stderr2.strip()} stdout={stdout2.strip()}")
            if rc2 != 0:
                print(f"  PATCH FAILED: git apply: {stderr.strip()}", file=sys.stderr)
                print(f"  PATCH FAILED: patch -p1: {stderr2.strip()}", file=sys.stderr)
                return write_score(
                    bench_name,
                    agent="control-solution",
                    resolved=False,
                    wall_clock=0.0,
                    control_type="solution-applied",
                )

        # Run the test
        passed, elapsed = run_test(container, workdir)
        score = write_score(
            bench_name,
            agent="control-solution",
            resolved=passed,
            wall_clock=elapsed,
            control_type="solution-applied",
        )
        return score
    finally:
        docker_rm(container)


def run_control(bench_name, dry_run=False):
    """Run both control arms for a single benchmark."""
    bench_dir = os.path.join(BENCHMARKS_DIR, bench_name)

    if not os.path.isdir(bench_dir):
        print(f"ERROR: benchmark not found: {bench_name}", file=sys.stderr)
        return None, None

    if dry_run:
        patch = os.path.join(bench_dir, ".bench", "solution.patch")
        has_patch = os.path.exists(patch)
        print(f"  [dry-run] Would build image nb-ctrl-{bench_name}")
        print(f"  [dry-run] Would run no-agent control (expect FAIL)")
        print(f"  [dry-run] Would run solution-applied control (expect PASS) — patch exists: {has_patch}")
        return None, None

    print(f"\n{'='*60}")
    print(f"  {bench_name}")
    print(f"{'='*60}")

    # Build Docker image
    print(f"  Building image...", end=" ", flush=True)
    if not docker_build(bench_name, bench_dir):
        return None, None
    print("done")

    # No-agent control
    print(f"  Running no-agent control...", end=" ", flush=True)
    baseline = run_no_agent(bench_name, bench_dir)
    if baseline:
        status = "PASS (UNEXPECTED)" if baseline["resolved"] else "FAIL (expected)"
        print(status)

    # Solution-applied control
    print(f"  Running solution-applied control...", end=" ", flush=True)
    solution = run_solution_applied(bench_name, bench_dir)
    if solution:
        status = "PASS (expected)" if solution["resolved"] else "FAIL (UNEXPECTED)"
        print(status)
    elif solution is None:
        print("SKIPPED (no patch)")

    return baseline, solution


def print_summary(results):
    """Print a summary table of all control results."""
    print(f"\n{'='*72}")
    print(f"  CONTROL SUMMARY")
    print(f"{'='*72}")
    print(f"{'Benchmark':<40} {'Baseline':>10} {'Solution':>10}")
    print(f"{'-'*40} {'-'*10} {'-'*10}")

    invalid_count = 0
    valid_count = 0
    skipped_count = 0

    for bench_name, (baseline, solution) in sorted(results.items()):
        if baseline is None and solution is None:
            skipped_count += 1
            print(f"{bench_name:<40} {'SKIP':>10} {'SKIP':>10}")
            continue

        # Baseline: should FAIL (resolved=False is correct)
        if baseline:
            baseline_ok = not baseline["resolved"]
            b_str = f"FAIL \u2713" if baseline_ok else f"PASS \u2717"
        else:
            b_str = "N/A"
            baseline_ok = False

        # Solution: should PASS (resolved=True is correct)
        if solution:
            solution_ok = solution["resolved"]
            s_str = f"PASS \u2713" if solution_ok else f"FAIL \u2717"
        else:
            s_str = "N/A"
            solution_ok = False

        is_valid = baseline_ok and solution_ok
        suffix = ""
        if not is_valid:
            suffix = "  <- INVALID"
            invalid_count += 1
        else:
            valid_count += 1

        print(f"{bench_name:<40} {b_str:>10} {s_str:>10}{suffix}")

    print(f"\n  Valid: {valid_count}  Invalid: {invalid_count}  Skipped: {skipped_count}")
    print(f"  Total: {valid_count + invalid_count + skipped_count}")


def main():
    parser = argparse.ArgumentParser(
        description="Run control arms for needle-bench benchmarks."
    )
    parser.add_argument(
        "--benchmark",
        help="Run a single benchmark by name",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would run without executing",
    )
    args = parser.parse_args()

    if args.benchmark:
        benchmarks = [args.benchmark]
    else:
        benchmarks = list_benchmarks()

    if args.dry_run:
        print(f"[dry-run] Would run {len(benchmarks)} benchmarks:\n")
        for bench in benchmarks:
            print(f"  {bench}")
            run_control(bench, dry_run=True)
        return

    print(f"Running control arms for {len(benchmarks)} benchmarks")
    print(f"Results will be written to: {RUNS_DIR}")

    results = {}
    for bench in benchmarks:
        baseline, solution = run_control(bench)
        results[bench] = (baseline, solution)

    print_summary(results)


if __name__ == "__main__":
    main()
