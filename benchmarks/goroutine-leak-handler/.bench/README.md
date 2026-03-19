# goroutine-leak-handler

## Difficulty
Medium

## Source
Community-submitted

## Environment
Go 1.21, Alpine Linux

## The bug
The HTTP handler in `app/main.go` spawns a goroutine for each `/compute` request but does not pass the request context or check for client disconnection. When a client times out, the goroutine continues running its computation loop indefinitely. The handler also uses `time.After` instead of `ctx.Done()` in its select, so it never learns the client left.

## Why Medium
Requires understanding Go's `context.Context` propagation, `http.Request.Context()`, and the select/channel pattern for cancellation. The fix spans the handler function and the computation function, requiring context threading through both. Not immediately obvious from test output alone.

## Expected fix
Pass `r.Context()` into the computation goroutine, replace the `time.After` fallback with `ctx.Done()`, and add a context cancellation check inside the computation loop so goroutines exit when clients disconnect.

## Pinned at
Anonymized snapshot, original repo not disclosed
