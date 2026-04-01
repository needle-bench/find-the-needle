# bidi-override-injection

## Difficulty
Extreme

## Source
Synthetic — modeled on real-world Unicode BIDI override attacks (CVE-2021-42574 "Trojan Source", GitHub username spoofing incidents, BIDI-based phishing in email clients)

## Environment
Python 3.12, Flask, Alpine Linux

## The bug
The application has two interacting flaws:

1. **Registration** (`app/sanitizer.py`): The `sanitize_username()` function strips HTML tags and escapes entities to prevent XSS, but does not strip Unicode control characters (categories Cc, Cf, Co, Cs). An attacker can register a username containing U+202E (RIGHT-TO-LEFT OVERRIDE) followed by "nimda", which stores the raw BIDI bytes.

2. **Authorization** (`app/auth.py`): The `check_admin_access()` function passes the session username through `_render_display_name()`, which processes BIDI override characters and reverses character order when U+202E is present. The string `\u202enimda` renders as `admin` after BIDI processing. This rendered form is compared against the `ADMIN_USERS` allowlist, granting the attacker admin privileges.

The XSS sanitizer in `sanitizer.py` acts as a red herring — it looks like comprehensive input validation but only addresses HTML injection, not Unicode control character injection. An agent that sees the sanitizer may assume input validation is handled and look elsewhere for the bug.

## Why Extreme
- Unicode BIDI override attacks are an obscure security vector that most developers and AI agents have never encountered.
- The XSS sanitizer red herring makes it look like input validation is already handled.
- The bug spans two files: the registration path (missing validation) and the auth middleware (wrong comparison strategy).
- The `_render_display_name()` function is well-commented and looks intentional — it describes its BIDI processing as a feature for "UI consistency", making it non-obvious that it is the authorization flaw.
- The agent must understand Unicode bidirectional algorithms, character categories, and the difference between stored vs. displayed string representations.
- Simply adding "nimda" to a blocklist would be a naive fix; the correct fix must handle all Unicode control characters and fix the auth comparison.

## Expected fix
Two changes required:

1. **`app/sanitizer.py`**: In `sanitize_username()`, add a step that strips all Unicode control characters using `unicodedata.category()` — remove characters in categories Cc (control), Cf (format, which includes BIDI overrides), Co (private use), and Cs (surrogate).

2. **`app/auth.py`**: In `check_admin_access()`, compare the raw stored `session["username"]` directly against `ADMIN_USERS`, instead of passing it through `_render_display_name()` first.

Both changes are needed: the sanitizer fix prevents new BIDI usernames from being registered, and the auth fix ensures existing sessions cannot exploit the display rendering.

## Pinned at
Synthetic benchmark, not from a real repository
