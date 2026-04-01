# relaxed-ordering-ringbuf

## Difficulty
Extreme

## Source
Community-submitted

## Environment
C11 (GCC), Alpine Linux, ThreadSanitizer

## The bug
The SPSC ring buffer uses `memory_order_relaxed` for all atomic loads and stores of `write_idx` and `read_idx`. On x86 (TSO), this happens to work because stores are never reordered with older stores. On ARM/POWER or under ThreadSanitizer, the consumer can see the updated `write_idx` before the producer's data write to the slot is visible, reading uninitialized or stale data. Similarly, the producer can see an updated `read_idx` before the consumer's read of the slot has completed, overwriting a slot still being read.

## Why Extreme
The program always produces correct output on x86 — the bug is completely masked by the hardware memory model. Only ThreadSanitizer catches the data race, and the agent must understand C11 memory ordering semantics (relaxed vs. acquire/release) to fix it. The presence of an `atomic_thread_fence(memory_order_seq_cst)` in the `ringbuf_resize()` function is a red herring: it looks like it addresses ordering concerns, but it only protects the single-threaded resize path, not the concurrent push/pop operations. The agent must reason about which specific atomic operations need acquire/release semantics and why, rather than cargo-culting `seq_cst` everywhere or adding fences in the wrong place. A naive "change all relaxed to seq_cst" would technically fix it but demonstrates no understanding.

## Expected fix
Change the producer's store of `write_idx` from `memory_order_relaxed` to `memory_order_release`, and the consumer's load of `write_idx` from `memory_order_relaxed` to `memory_order_acquire`. Similarly, change the consumer's store of `read_idx` to `memory_order_release` and the producer's load of `read_idx` to `memory_order_acquire`. This establishes the correct happens-before relationships: the data write happens-before the index publish (release), and the index observation happens-before the data read (acquire).

## Pinned at
Anonymized snapshot, original repo not disclosed
