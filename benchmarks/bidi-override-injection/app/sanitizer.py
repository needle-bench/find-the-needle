"""Input sanitization utilities.

Provides functions to clean user-supplied strings before storage or display.
Used across the application for XSS prevention and general input hygiene.
"""

import re
import html


# Matches any HTML tag (opening, closing, self-closing)
_HTML_TAG_RE = re.compile(r"<[^>]+>")

# Matches common script injection patterns
_SCRIPT_RE = re.compile(
    r"(javascript\s*:|on\w+\s*=|<\s*script)",
    re.IGNORECASE,
)

# Dangerous URI schemes
_DANGEROUS_SCHEMES = {"javascript", "vbscript", "data"}


def sanitize_username(username: str) -> str:
    """Sanitize a username for safe storage and display.

    Strips HTML tags and script injection vectors to prevent XSS.
    Limits length to 64 characters and lowercases for consistency.
    """
    # Step 1: HTML-escape any special characters
    cleaned = html.escape(username, quote=True)

    # Step 2: Strip any residual HTML tags (belt-and-suspenders)
    cleaned = _HTML_TAG_RE.sub("", cleaned)

    # Step 3: Reject script injection patterns
    if _SCRIPT_RE.search(cleaned):
        raise ValueError("Username contains disallowed script patterns")

    # Step 4: Enforce length limits
    cleaned = cleaned[:64]

    # Step 5: Normalize whitespace
    cleaned = cleaned.strip()
    if not cleaned:
        raise ValueError("Username cannot be empty")

    return cleaned


def sanitize_display_text(text: str) -> str:
    """Sanitize arbitrary text for display in HTML context.

    Escapes HTML entities and removes dangerous tags.
    """
    escaped = html.escape(text, quote=True)
    return _HTML_TAG_RE.sub("", escaped)


def is_safe_redirect(url: str) -> bool:
    """Check if a redirect URL is safe (no open redirect)."""
    from urllib.parse import urlparse
    parsed = urlparse(url)
    # Only allow relative URLs or same-origin
    if parsed.scheme and parsed.scheme.lower() in _DANGEROUS_SCHEMES:
        return False
    if parsed.netloc:
        return False
    return True
