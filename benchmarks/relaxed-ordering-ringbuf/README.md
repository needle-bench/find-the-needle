# relaxed-ordering-ringbuf

## Project

A C11 lock-free single-producer single-consumer (SPSC) ring buffer used in a low-latency data pipeline. The producer thread enqueues sequential integers; the consumer thread dequeues and verifies them. The implementation uses `<stdatomic.h>` atomics for the read and write indices to avoid mutex overhead.

## Symptoms

The program compiles without warnings and produces **correct output** every time it runs. All items are transferred in order, and the functional correctness check passes. However, when the code is compiled with ThreadSanitizer (`-fsanitize=thread`), TSan reports data races on the ring buffer operations. The `test.sh` script fails because TSan exits non-zero when it detects these races.

## Bug description

The ring buffer's atomic operations use incorrect memory ordering. The data race is real but masked by x86's strong memory model (Total Store Order). On weaker memory architectures (ARM, POWER) or under TSan's instrumentation, the consumer can observe a stale or reordered write index relative to the actual data store, potentially reading uninitialized buffer slots. The fix requires understanding the C11 memory model and the acquire/release semantics needed for producer-consumer synchronization.

## Difficulty

Extreme
