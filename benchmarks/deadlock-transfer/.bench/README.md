# deadlock-transfer

## Difficulty
Medium

## Source
Community-submitted

## Environment
Java 17, Alpine Linux

## The bug
The `TransferService.java` acquires locks in source-then-destination order. When two threads simultaneously transfer between the same accounts in opposite directions (A->B and B->A), each acquires one lock and waits for the other, causing a classic ABBA deadlock. The program hangs until the timeout fires.

## Why Medium
Requires understanding concurrent lock acquisition ordering and the ABBA deadlock pattern. The test runs 200 transfers across 8 threads, so the deadlock is reliably triggered but the root cause is not visible in the output -- the program simply hangs. Understanding synchronized blocks and lock ordering is required.

## Expected fix
Impose a consistent lock ordering by always acquiring the lock on the account with the smaller ID first, regardless of transfer direction.

## Pinned at
Anonymized snapshot, original repo not disclosed
