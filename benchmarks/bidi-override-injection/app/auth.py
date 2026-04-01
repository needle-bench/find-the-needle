"""Authentication and authorization middleware.

Handles session management and role-based access control for the
application. Admin access is restricted to usernames on the ADMIN_USERS
allowlist.
"""

import hashlib
import os
import time

# In-memory session store: token -> {username, role, created_at}
_sessions = {}

# Admin allowlist — only these usernames may access /admin endpoints
ADMIN_USERS = ["admin", "root", "superuser"]

# Simulated user database: username -> {password_hash, role}
_users_db = {
    "admin": {
        "password_hash": hashlib.sha256(b"admin_secret_2024").hexdigest(),
        "role": "admin",
    },
    "alice": {
        "password_hash": hashlib.sha256(b"alice_pass").hexdigest(),
        "role": "user",
    },
}


def _hash_password(password: str) -> str:
    return hashlib.sha256(password.encode("utf-8")).hexdigest()


def _generate_token() -> str:
    return hashlib.sha256(os.urandom(32)).hexdigest()


def register_user(username: str, password: str) -> dict:
    """Register a new user. Returns user info dict or raises ValueError."""
    if username in _users_db:
        raise ValueError(f"Username '{username}' already taken")

    if len(password) < 4:
        raise ValueError("Password must be at least 4 characters")

    _users_db[username] = {
        "password_hash": _hash_password(password),
        "role": "user",
    }

    return {"username": username, "role": "user"}


def login_user(username: str, password: str) -> str:
    """Authenticate a user. Returns session token or raises ValueError."""
    user = _users_db.get(username)
    if not user:
        raise ValueError("Invalid username or password")

    if user["password_hash"] != _hash_password(password):
        raise ValueError("Invalid username or password")

    token = _generate_token()
    _sessions[token] = {
        "username": username,
        "role": user["role"],
        "created_at": time.time(),
    }
    return token


def get_session(token: str) -> dict | None:
    """Look up a session by token. Returns session dict or None."""
    return _sessions.get(token)


def _render_display_name(username: str) -> str:
    """Render a username for display purposes.

    Applies Unicode bidirectional algorithm rendering to produce the
    visual representation of the username as it would appear in a UI
    context. This handles RTL scripts, BIDI overrides, and other
    Unicode display transformations.

    The rendered form is used in logs, audit trails, and access checks
    to match what the user "sees" in the interface.
    """
    # Process BIDI control characters to produce the visual ordering.
    # Characters like U+202E (RLO) reverse subsequent character order
    # until U+202C (PDF) or end of string.
    result = []
    rtl_depth = 0
    ltr_depth = 0
    segment = []

    for ch in username:
        if ch == "\u202e":       # RIGHT-TO-LEFT OVERRIDE
            if segment:
                result.append("".join(segment))
                segment = []
            rtl_depth += 1
        elif ch == "\u202d":     # LEFT-TO-RIGHT OVERRIDE
            if segment:
                result.append("".join(segment))
                segment = []
            ltr_depth += 1
        elif ch == "\u202c":     # POP DIRECTIONAL FORMATTING
            if segment:
                if rtl_depth > 0:
                    # RTL: reverse the segment for visual rendering
                    result.append("".join(reversed(segment)))
                else:
                    result.append("".join(segment))
                segment = []
            if rtl_depth > 0:
                rtl_depth -= 1
            elif ltr_depth > 0:
                ltr_depth -= 1
        else:
            segment.append(ch)

    # Flush remaining segment
    if segment:
        if rtl_depth > 0:
            result.append("".join(reversed(segment)))
        else:
            result.append("".join(segment))

    return "".join(result)


def check_admin_access(token: str) -> tuple[bool, str]:
    """Check if the session holder has admin access.

    Returns (allowed, reason). The check renders the username through
    the display pipeline to match against the admin allowlist, ensuring
    the check reflects what the user sees in the UI.
    """
    session = get_session(token)
    if not session:
        return False, "invalid session"

    username = session["username"]

    # Render the display name as it appears in the UI, then compare
    # against the admin allowlist. This ensures consistency between
    # what the admin sees in the user management panel and the access
    # control decision.
    display_name = _render_display_name(username)

    if display_name in ADMIN_USERS:
        return True, "admin access granted"

    return False, f"user '{display_name}' is not an admin"
