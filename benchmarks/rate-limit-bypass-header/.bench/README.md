# rate-limit-bypass-header

## Difficulty
Medium

## Source
Community-submitted

## Environment
Node.js 20, TypeScript, Alpine Linux

## The bug
The rate limiter in `app/ratelimiter.ts` identifies clients using the `X-Forwarded-For` header when present, falling back to the socket address. An attacker can rotate the `X-Forwarded-For` value on each request to get a fresh rate limit window every time, effectively bypassing the rate limiter entirely.

## Why Medium
Requires understanding HTTP proxy headers, the trust model for X-Forwarded-For, and how rate limiting relies on stable client identification. The agent must reason about the security implications of trusting client-supplied headers for security-critical decisions.

## Expected fix
Remove the X-Forwarded-For header trust logic from `getClientIp()` and always use the socket's remote address for rate limiting, since the server is not behind a trusted reverse proxy.

## Pinned at
Anonymized snapshot, original repo not disclosed
