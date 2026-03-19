# k8s-scheduler-shutdown-deadlock

## Difficulty
Hard

## Source
Kernel-curated from kubernetes/kubernetes (2025-03-01)

## Environment
Go 1.24, Debian Linux

## The bug
In `main.go`, the scheduler's `Run()` method waits for the ScheduleOne goroutine to finish (`<-done`) before calling `queue.Close()`. But ScheduleOne is blocked inside `queue.Pop()` which holds the internal lock. `Close()` needs that same lock to signal the queue to stop. This creates a deadlock: Run waits for the goroutine, which waits for Pop, which waits for Close, which waits for Run.

## Why Hard
Requires understanding Go concurrency, channel/goroutine lifecycle, and mutex contention across goroutine boundaries. The deadlock is in the shutdown ordering -- a subtle interaction between two goroutines and a shared mutex. The program simply hangs with no error output, requiring the agent to reason about what's blocking.

## Expected fix
Swap the ordering: call `queue.Close()` before waiting on the done channel, so the queue is signaled to stop before blocking on the goroutine's completion.

## Pinned at
kubernetes/kubernetes@pkg/scheduler/scheduler.go (one state before fix)
