# needle-bench harness layer — installs ostk + native CLI harnesses.
#
# Usage: add to benchmark Dockerfiles as a build stage or COPY --from.
#   FROM needle-bench-harness AS harness
#   COPY --from=harness /usr/local/bin/ostk /usr/local/bin/ostk
#
# Or source this as a multi-stage layer in benchmark Dockerfiles:
#   RUN curl -fsSL https://ostk.ai/install | sh
#
# This file documents what each harness needs. Benchmark Dockerfiles
# should install only what they need (not everything).

# ── ostk (treatment arm — all models) ────────────────────────────────
#
# Install ostk binary from release:
#   curl -fsSL https://ostk.ai/install | OSTK_INSTALL_DIR=/usr/local/bin sh
#
# Initialize in workdir:
#   ostk init --non-interactive
#
# Required env: ANTHROPIC_API_KEY or OPENROUTER_API_KEY (for the model)

# ── Claude Code (bare arm — Anthropic models) ────────────────────────
#
# Requires: Node.js >= 18
#   npm install -g @anthropic-ai/claude-code
#
# Run:
#   claude -p "find the needle. run test.sh to verify." \
#     --model claude-sonnet-4-6 \
#     --permission-mode acceptEdits \
#     --max-turns 40 \
#     --output-format json
#
# Required env: ANTHROPIC_API_KEY

# ── Gemini CLI (bare arm — Google models) ─────────────────────────────
#
# Requires: Node.js >= 18
#   npm install -g @anthropic-ai/gemini-cli  # TODO: verify package name
#
# Run:
#   gemini -p "find the needle. run test.sh to verify."
#
# Required env: GEMINI_API_KEY

# ── Codex CLI (bare arm — OpenAI models) ──────────────────────────────
#
# Requires: Node.js >= 18
#   npm install -g @openai/codex
#
# Run:
#   codex -p "find the needle. run test.sh to verify."
#
# Required env: OPENAI_API_KEY

# ── Fallback (bare arm — all other models) ────────────────────────────
#
# Uses runner.py's simple agent loop (API call → tool execution → repeat).
# No harness installed — just bash/read/edit tools via docker exec.
# Required env: OPENROUTER_API_KEY
