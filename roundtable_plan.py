#!/usr/bin/env python3
"""Roundtable review of the needle system upgrade plan."""
import json, os, urllib.request, textwrap

OPENROUTER_API_KEY = os.environ.get("OPENROUTER_API_KEY") or open("/tmp/.or_key").read().strip()
ENDPOINT = "https://openrouter.ai/api/v1/chat/completions"

PLAN = open(os.path.expanduser("~/.claude/plans/goofy-conjuring-forest.md")).read()

PARTICIPANTS = [
    {"name": "gemini-2.5-pro", "model": "google/gemini-2.5-pro",
     "persona": "You are a systems architect reviewing a plan for upgrading a work management system inside an AI coordination kernel (ostk/haystack). You specialize in graph data models, dependency resolution, and developer tooling. Be critical — find gaps, missing edge cases, and architectural concerns."},
    {"name": "claude-sonnet-4.5", "model": "anthropic/claude-sonnet-4-5",
     "persona": "You are a senior engineer reviewing a plan for upgrading a needle (task) system. You care about: backward compatibility, migration paths, simplicity over complexity, and whether the abstractions actually help users. Push back on over-engineering. Be practical."},
]

ROUND_PROMPTS = [
    textwrap.dedent("""\
    Review this plan for upgrading ostk's needle (task management) system. The system runs inside an AI coordination kernel that manages fleets of AI agents.

    {persona}

    THE PLAN:
    {plan}

    ROUND 1: What gaps do you see? What could go wrong? What's missing? What's over-engineered? Be specific. 3-4 paragraphs."""),

    textwrap.dedent("""\
    ROUND 2: You've heard the other reviewer's feedback. React, add what they missed, and synthesize. What are the top 3 changes you'd make to this plan?

    {prev_responses}

    2-3 paragraphs."""),

    textwrap.dedent("""\
    ROUND 3 (final): If you had to prioritize the 5 phases, which order would you build them and why? What's the minimum viable version that delivers value? What can wait?

    {prev_responses}

    2-3 paragraphs."""),
]

def call_model(model, messages):
    payload = json.dumps({
        "model": model,
        "messages": messages,
        "max_tokens": 1500,
        "temperature": 0.7,
    }).encode()
    req = urllib.request.Request(ENDPOINT, data=payload, headers={
        "Authorization": f"Bearer {OPENROUTER_API_KEY}",
        "Content-Type": "application/json",
    })
    resp = json.loads(urllib.request.urlopen(req, timeout=120).read())
    return resp["choices"][0]["message"]["content"]

def run():
    history = {p["name"]: [] for p in PARTICIPANTS}

    for round_num in range(3):
        print(f"\n{'='*70}")
        print(f"  ROUND {round_num + 1}")
        print(f"{'='*70}")

        for p in PARTICIPANTS:
            if round_num == 0:
                prompt = ROUND_PROMPTS[0].format(persona=p["persona"], plan=PLAN)
            else:
                others = [f"**{name}** (Round {round_num}):\n{history[name][-1]}"
                          for name in history if name != p["name"] and history[name]]
                prev = "\n\n---\n\n".join(others)
                prompt = ROUND_PROMPTS[round_num].format(prev_responses=prev)

            messages = [{"role": "user", "content": prompt}]
            print(f"\n--- {p['name']} ---\n")
            try:
                response = call_model(p["model"], messages)
                print(response)
                history[p["name"]].append(response)
            except Exception as e:
                msg = f"[ERROR: {e}]"
                print(msg)
                history[p["name"]].append(msg)

    print(f"\n{'='*70}")
    print(f"  ROUNDTABLE COMPLETE")
    print(f"{'='*70}")

if __name__ == "__main__":
    run()
