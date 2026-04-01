/*
 * ring_buffer.h — Lock-free single-producer single-consumer (SPSC) ring buffer.
 *
 * Design notes:
 *   - Power-of-two capacity for fast modular indexing (bitwise AND).
 *   - Atomic indices enable wait-free push/pop without mutexes.
 *   - resize() protected by a full memory fence to ensure internal
 *     consistency of the capacity/mask fields during reallocation.
 *   - Designed for embedded/low-latency pipelines where throughput
 *     matters more than generality (single producer, single consumer).
 */

#ifndef RING_BUFFER_H
#define RING_BUFFER_H

#include <stdatomic.h>
#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

#define RINGBUF_DEFAULT_CAPACITY 4096  /* must be power of 2 */

typedef struct {
    int64_t *slots;
    size_t   capacity;     /* always a power of 2 */
    size_t   mask;         /* capacity - 1, for fast modulo */

    /*
     * Atomic indices: write_idx is only modified by producer,
     * read_idx is only modified by consumer. Both are read by
     * the other side to check full/empty conditions.
     *
     * NOTE: We use relaxed ordering throughout for maximum throughput.
     * On x86/x86-64 (TSO architecture), stores are never reordered
     * with older stores, so relaxed is sufficient. See Intel SDM
     * Vol. 3A Section 8.2.3.
     */
    _Alignas(64) atomic_size_t write_idx;
    _Alignas(64) atomic_size_t read_idx;
} ring_buffer_t;

/* Lifecycle */
ring_buffer_t *ringbuf_create(size_t capacity);
void           ringbuf_destroy(ring_buffer_t *rb);

/* Producer API — call from producer thread only */
bool ringbuf_push(ring_buffer_t *rb, int64_t value);

/* Consumer API — call from consumer thread only */
bool ringbuf_pop(ring_buffer_t *rb, int64_t *out);

/* Utility */
size_t ringbuf_size(ring_buffer_t *rb);
bool   ringbuf_empty(ring_buffer_t *rb);
bool   ringbuf_full(ring_buffer_t *rb);

/*
 * Resize the ring buffer (NOT thread-safe — must be called when
 * no concurrent push/pop is in progress). Used during initial
 * calibration or reconfiguration phases.
 */
bool ringbuf_resize(ring_buffer_t *rb, size_t new_capacity);

#endif /* RING_BUFFER_H */
