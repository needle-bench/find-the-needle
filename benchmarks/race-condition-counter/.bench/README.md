# race-condition-counter

## Difficulty
Medium

## Source
Community-submitted

## Environment
Node.js 20, Redis, Alpine Linux

## The bug
The increment function in `app/counter.js` performs a non-atomic read-modify-write: it reads the current value with `GET`, increments in application code, then writes back with `SET`. Under concurrent load, multiple requests read the same value before any writes land, causing lost updates.

## Why Medium
Requires understanding the read-modify-write race condition pattern and Redis's atomic operations. The agent must recognize that the sequential GET/increment/SET is not safe under concurrency and know that Redis provides an atomic INCR command. The test fires 100 concurrent increments to reliably expose the race.

## Expected fix
Replace the GET + increment + SET sequence with Redis's atomic `INCR` command, which performs the read-modify-write in a single atomic operation server-side.

## Pinned at
Anonymized snapshot, original repo not disclosed
