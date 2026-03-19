# Contributing to needle-bench

## Two ways to contribute

### 1. Submit a benchmark (Pipeline 1)

Turn your worst debugging day into a benchmark.

**Requirements:**
- `Dockerfile` — builds the broken environment
- `test.sh` — exits non-zero when the bug is present, zero when fixed
- `.bench/solution.patch` — the actual fix
- `.bench/README.md` — describes the bug, environment, and difficulty rating
- Source code in `app/` or `src/` with the bug present

**Process:**
1. Copy `benchmarks/_template/` to `benchmarks/your-scenario-name/`
2. Build your scenario, verify `test.sh` fails without the patch and passes with it
3. Write `.bench/README.md` using the template
4. Open a PR — CI validates automatically
5. A maintainer reviews the difficulty rating and merges

### 2. Improve the framework

Bug fixes, scoring improvements, leaderboard features, runner enhancements — all welcome via PR.

---

## Upstream contributions (Pipeline 2)

When the weekly kernel-curated pipeline creates a benchmark from a real open-source repo and a model solves it, we offer the fix upstream. These contributions follow strict guidelines.

### Before opening an upstream PR

1. **The fix goes to the os-tack fork first.** Never PR directly to upstream from automation.
2. **A human reviews the fix.** Model-generated patches must be verified by a maintainer before upstream submission.
3. **Check upstream's CONTRIBUTING.md.** Every project has its own rules — follow them exactly.
4. **One fix per PR.** Atomic commits, no drive-by cleanups, no "while I'm here" changes.
5. **Run upstream's CI locally** if possible. Don't waste maintainer time on broken patches.

### PR format for upstream contributions

```
Title: fix: <concise description of the bug>

Body:
## Summary
<1-2 sentences describing the fix>

## Context
This fix was discovered by [needle-bench](https://needle-bench.cc), an open
benchmark for AI debugging agents. The issue was identified by the
[haystack](https://ostk.ai) kernel as a high-leverage fix in this codebase.

Solved by: <model-name> (e.g. claude-opus-4-6)
Benchmark: <benchmark-name>
Leaderboard: https://needle-bench.cc/leaderboard/

## The bug
<description of what was wrong and why>

## The fix
<description of what changed and why it's correct>

## Testing
<how the fix was verified — test output, CI results>
```

### What we do NOT do

- **No spam.** One PR per diagnosed issue, only when the fix is verified and correct.
- **No trivial PRs.** The weekly pipeline selects the most compounding issue — not typos, not style nits.
- **No unsolicited refactoring.** The fix addresses one bug, nothing more.
- **No pressure.** If upstream closes or ignores the PR, that's their prerogative. The benchmark value was already captured.
- **No claiming credit beyond attribution.** The model found it, the benchmark measured it, the project owns it.

### Attribution

Every upstream PR includes:
- The model that solved the benchmark
- A link to needle-bench
- A link to the specific benchmark (Dockerfile, test.sh, Agentfile)

The project and its maintainers receive full credit for their codebase. We contribute a fix. That's it.

---

## Code style

- Python: follow existing patterns in `runner.py` and `pipeline/`
- Astro/JS: follow patterns in `src/`
- No over-engineering — keep changes minimal and focused
- Tests pass, CI green, Astro builds

## Questions?

Open an issue at [os-tack/find-the-needle](https://github.com/os-tack/find-the-needle/issues).
