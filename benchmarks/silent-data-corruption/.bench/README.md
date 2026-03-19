# silent-data-corruption

## Difficulty
Medium

## Source
Community-submitted

## Environment
Rust 1.82, Alpine Linux

## The bug
The file processor in `src/main.rs` reads input into a fixed 64KB buffer using `file.read(&mut buffer)`, which only reads up to the buffer size. For files larger than 64KB, the remainder is silently discarded. The tool reports success and writes the truncated output without any error.

## Why Medium
Requires understanding Rust's `Read::read()` semantics (single read, not guaranteed to read all bytes) versus `read_to_end()`. The truncation is silent -- no error is reported -- so the agent must reason about why the output is smaller than the input. The 64KB buffer size is a non-obvious threshold.

## Expected fix
Replace the fixed-size buffer with a `Vec<u8>` and use `read_to_end()` to read the entire file regardless of size.

## Pinned at
Anonymized snapshot, original repo not disclosed
