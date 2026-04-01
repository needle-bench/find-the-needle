# bidi-override-injection

## Project

A Python Flask web application with user registration, login, and an admin dashboard protected by role-based access control. Usernames are sanitized during registration to prevent injection attacks, and an authorization middleware checks session usernames against an admin allowlist before granting access to privileged endpoints.

## Symptoms

A security audit has revealed that an unauthorized user is accessing the admin dashboard and retrieving sensitive system configuration (database credentials, API keys, internal endpoints). The application logs show a user session with admin privileges, but the username does not appear in the admin allowlist when viewed in certain terminals or log viewers. The XSS sanitization on registration appears to be working correctly — no HTML tags or script injections are getting through.

## Bug description

The registration endpoint sanitizes usernames for XSS (stripping HTML tags, escaping entities) but does not strip Unicode control characters. An attacker registers the username `\u202enimda` — the U+202E character is a RIGHT-TO-LEFT OVERRIDE that causes subsequent characters to render in reverse order. The stored username contains the raw BIDI bytes, but when the authorization middleware renders it through a "display name" pipeline (which processes BIDI control characters), the visual result is `admin`. The admin check compares this display-rendered name against the allowlist, granting the attacker admin access.

## Difficulty

Extreme
