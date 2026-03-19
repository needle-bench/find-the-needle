# timing-attack-comparison

## Difficulty
Hard

## Source
Community-submitted

## Environment
Go 1.22, Alpine Linux

## The bug
The hash comparison function in `app/auth/compare.go` uses Go's `==` string equality operator, which short-circuits on the first differing byte. This creates a timing side-channel: inputs whose hashes share a longer prefix with the stored hash take measurably longer to compare, allowing an attacker to iteratively guess the hash one byte at a time.

## Why Hard
Requires understanding timing side-channel attacks and constant-time comparison. The basic auth tests pass -- the bug is purely a security property, not a functional one. The agent must recognize that `==` comparison leaks information through timing and know to use `crypto/subtle.ConstantTimeCompare` as the replacement.

## Expected fix
Replace the `==` string comparison with `crypto/subtle.ConstantTimeCompare` which always examines every byte regardless of match position.

## Pinned at
Anonymized snapshot, original repo not disclosed
