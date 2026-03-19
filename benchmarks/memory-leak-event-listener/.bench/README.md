# memory-leak-event-listener

## Difficulty
Medium

## Source
Community-submitted

## Environment
Node.js 20, Alpine Linux

## The bug
The `process()` method in `app/processor.js` registers new `data:validated` and `data:error` event listeners on the shared event bus on every call. These listeners are never removed, so after N requests there are N listener closures retained in memory, each capturing its request-scoped data and preventing garbage collection.

## Why Medium
Requires understanding Node.js EventEmitter lifecycle and closure retention. The bug is not in the obvious request path but in how event-driven architecture interacts with per-request state. The fix requires restructuring the event flow to avoid per-request listener registration.

## Expected fix
Remove the per-request `eventBus.on()` calls from `process()`. Instead, call `_handleValidated()` directly after emitting the validated data, and handle errors inline rather than through event listeners.

## Pinned at
Anonymized snapshot, original repo not disclosed
