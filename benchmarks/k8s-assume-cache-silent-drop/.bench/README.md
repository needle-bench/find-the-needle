# k8s-assume-cache-silent-drop

## Difficulty
Hard

## Source
Kernel-curated from kubernetes/kubernetes (2025-03-01)

## Environment
Go 1.24, Debian Linux

## The bug
In `main.go`, the `informerUpdate()` method silently deletes assumed entries when an informer delivers a newer version of the object. The caller that originally called `Assume()` is never notified that its optimistic state was overwritten. This is a missing conflict notification in an optimistic concurrency control (OCC) pattern -- the "conflict" half of OCC is absent.

## Why Hard
Requires understanding optimistic concurrency control, the Kubernetes scheduler's assume cache pattern, and the informer/cache interaction model. The agent must recognize that the silent `delete(c.assumed, obj.Key)` is the bug, understand why a callback is needed, add an `onConflict` callback field to the cache struct, invoke it before deletion, and wire it up in `main()` to set `conflictNotified = true`.

## Expected fix
Add an `onConflict` callback function to the AssumeCache struct, invoke it in `informerUpdate()` before deleting the assumed entry, and register a callback in `main()` that sets `conflictNotified = true`.

## Pinned at
kubernetes/kubernetes@pkg/scheduler/framework/plugins/volumebinding/assume_cache.go (one state before fix)
