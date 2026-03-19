# auth-bypass-path-traversal

## Difficulty
Medium

## Source
Community-submitted

## Environment
Go 1.21, Alpine Linux

## The bug
The auth middleware in `app/middleware.go` checks the raw `r.URL.Path` for the `/api/` prefix to decide if authentication is required. However, Go's HTTP router normalizes paths (collapses `//`, resolves `..`) before matching handlers. An attacker can use `//api/admin` or `/health/../api/admin` to bypass the prefix check while the router still routes to the protected handler.

## Why Medium
Requires understanding the gap between middleware path inspection and router path normalization. The agent must reason about how different URL paths are seen at different stages of the request pipeline. Multiple path manipulation techniques must be defended against.

## Expected fix
Normalize the request path in the middleware (using `path.Clean`) before checking the `/api/` prefix, so the middleware sees the same path the router will match.

## Pinned at
Anonymized snapshot, original repo not disclosed
