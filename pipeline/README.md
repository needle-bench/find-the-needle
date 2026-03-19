# needle-bench Pipelines

needle-bench has two pipelines that feed the same leaderboard. Both produce
Docker-based benchmarks in the `benchmarks/` directory with the same format
(Dockerfile, Agentfile, test.sh, .bench/solution.patch).

## Pipeline 1: Community-Submitted

Manually curated benchmarks contributed by the community.

- Benchmarks are hand-crafted with known bugs, test harnesses, and solutions.
- Submitted via PR, validated by CI (`ci.yml` builds the Docker image and
  verifies that applying `solution.patch` makes `test.sh` pass).
- Difficulty is assigned by human reviewers in `difficulty.json`.
- Best for targeted, high-quality benchmarks with well-understood root causes.

## Pipeline 2: Kernel-Curated (Weekly)

Automated weekly pipeline (`weekly.py`) that imports a well-known open-source
repo, uses ostk to diagnose the most compounding issue, and creates a
frozen benchmark from it.

### How it works

1. **Import** — Shallow-clone a repo from the curated list (`repos.json`).
   Rotation is deterministic: `week_number % len(repos)`.
2. **Diagnose** — Run ostk on the repo to produce a ranked list of
   "needles" (compounding issues).
3. **Select** — Pick the highest-leverage needle (P0, most dependencies).
4. **Freeze** — Create a benchmark directory with Dockerfile, test.sh,
   and .bench/README.md pinned to the exact commit.
5. **Review** — A PR is opened for human review before the benchmark goes live.
6. **Upstream** — When a model solves the benchmark, the fix is offered
   upstream via PR with full attribution.

### Running manually

```bash
# Specific repo
python3 pipeline/weekly.py --repo django/django

# Auto-select by week number
python3 pipeline/weekly.py --auto

# List curated repos
python3 pipeline/weekly.py --list-repos

# Dry run (no cloning or file creation)
python3 pipeline/weekly.py --repo django/django --dry-run
```

### GitHub Actions

The `weekly-needle.yml` workflow runs every Monday at 6am UTC. It can also
be triggered manually with a specific repo via `workflow_dispatch`.

## How Both Pipelines Feed the Leaderboard

```
Pipeline 1 (community) ─┐
                         ├─> benchmarks/ ──> runner.py ──> scores ──> leaderboard
Pipeline 2 (weekly)    ──┘
```

Every benchmark in `benchmarks/` is registered in `difficulty.json` with a
tier (easy / medium / hard). The runner (`runner.py`) evaluates models against
all benchmarks uniformly — it does not distinguish between pipeline sources.

Scores are written to `runs/<model>/<benchmark>/` and consolidated into the
public leaderboard.

## How Fixes Flow Upstream

When Pipeline 2 creates a benchmark from a real open-source repo, and a model
subsequently solves it, the fix can be offered back to the original project:

1. Model solves benchmark, producing a passing patch.
2. `offer_fix()` forks the upstream repo, applies the patch, and opens a PR.
3. PR includes attribution: model name, benchmark ID, and link to needle-bench.
4. Upstream maintainers review and merge (or not) on their own terms.

This closes the loop: ostk finds the bug, needle-bench benchmarks it,
models compete to fix it, and the best fix goes back to the project.
