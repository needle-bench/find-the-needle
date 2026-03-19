# data-corruption-concurrent-write

## Difficulty
Hard

## Source
Community-submitted

## Environment
Rust 1.82, Alpine Linux

## The bug
The segment assignment function in `src/config.rs` adds an overlap of 2 segments per worker (`end = start + segments_per_worker + 2`) under the guise of "write redundancy." This causes adjacent workers to write to the same byte ranges with different fill patterns. Depending on thread scheduling, one worker's writes overwrite another's, producing non-deterministic corruption at segment boundaries.

## Why Hard
The corruption is non-deterministic and requires running multiple times to trigger. The agent must understand the concurrent write architecture, trace the segment assignment math to discover the overlap, and reason about why overlapping assignments cause data races. The "write redundancy" comment is misleading, making it harder to identify as a bug rather than a feature.

## Expected fix
Remove the `+ 2` overlap from the segment assignment calculation so each worker gets exactly its own non-overlapping segments.

## Pinned at
Anonymized snapshot, original repo not disclosed
