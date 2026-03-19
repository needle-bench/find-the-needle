# null-pointer-config

## Difficulty
Medium

## Source
Community-submitted

## Environment
Go 1.21, Alpine Linux

## The bug
The config loader in `app/config.go` does not initialize the `Metrics` sub-struct when the `enable_metrics` feature flag is true and no explicit metrics config is provided in the JSON. The `/status` handler dereferences `cfg.Metrics` (a nil pointer), causing a panic on the first request.

## Why Medium
Requires tracing the config loading path and understanding how Go's JSON unmarshaling leaves pointer fields as nil when absent from input. The agent must connect the nil pointer panic at runtime to the missing initialization logic in config loading. The fix requires understanding which default values to provide.

## Expected fix
Add a nil check after config loading: if `EnableMetrics` is true and `Metrics` is nil, initialize it with sensible defaults (endpoint, interval).

## Pinned at
Anonymized snapshot, original repo not disclosed
