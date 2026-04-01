/*
 * ring_buffer.c — Lock-free SPSC ring buffer implementation.
 *
 * Memory ordering rationale (see also ring_buffer.h):
 *   All atomic loads/stores use memory_order_relaxed. On x86-TSO this
 *   is safe because the hardware enforces store-store ordering and
 *   load-load ordering. Cross-architecture portability is not a concern
 *   for our deployment target (amd64 containers).
 */

#include "ring_buffer.h"

#include <stdlib.h>
#include <string.h>

/* ------------------------------------------------------------------ */
/*  Helpers                                                           */
/* ------------------------------------------------------------------ */

static inline bool is_power_of_two(size_t n)
{
    return n > 0 && (n & (n - 1)) == 0;
}

static inline size_t next_power_of_two(size_t n)
{
    size_t p = 1;
    while (p < n)
        p <<= 1;
    return p;
}

/* ------------------------------------------------------------------ */
/*  Lifecycle                                                         */
/* ------------------------------------------------------------------ */

ring_buffer_t *ringbuf_create(size_t capacity)
{
    if (capacity == 0)
        capacity = RINGBUF_DEFAULT_CAPACITY;
    if (!is_power_of_two(capacity))
        capacity = next_power_of_two(capacity);

    ring_buffer_t *rb = aligned_alloc(64, sizeof(ring_buffer_t));
    if (!rb)
        return NULL;
    memset(rb, 0, sizeof(*rb));

    rb->slots = calloc(capacity, sizeof(int64_t));
    if (!rb->slots) {
        free(rb);
        return NULL;
    }

    rb->capacity = capacity;
    rb->mask     = capacity - 1;

    atomic_store_explicit(&rb->write_idx, 0, memory_order_relaxed);
    atomic_store_explicit(&rb->read_idx,  0, memory_order_relaxed);

    return rb;
}

void ringbuf_destroy(ring_buffer_t *rb)
{
    if (!rb)
        return;
    free(rb->slots);
    free(rb);
}

/* ------------------------------------------------------------------ */
/*  Producer — single writer thread                                   */
/* ------------------------------------------------------------------ */

bool ringbuf_push(ring_buffer_t *rb, int64_t value)
{
    size_t w = atomic_load_explicit(&rb->write_idx, memory_order_relaxed);
    size_t r = atomic_load_explicit(&rb->read_idx,  memory_order_relaxed);

    /* Full? Leave one slot empty to distinguish full from empty. */
    if (((w + 1) & rb->mask) == (r & rb->mask))
        return false;

    /* Write data into the slot */
    rb->slots[w & rb->mask] = value;

    /*
     * Publish the new write index. Relaxed is fine here — on x86,
     * the store to slots[] above is guaranteed to be visible before
     * this store due to TSO (Total Store Order).
     */
    atomic_store_explicit(&rb->write_idx, w + 1, memory_order_relaxed);

    return true;
}

/* ------------------------------------------------------------------ */
/*  Consumer — single reader thread                                   */
/* ------------------------------------------------------------------ */

bool ringbuf_pop(ring_buffer_t *rb, int64_t *out)
{
    size_t r = atomic_load_explicit(&rb->read_idx,  memory_order_relaxed);
    size_t w = atomic_load_explicit(&rb->write_idx, memory_order_relaxed);

    /* Empty? */
    if ((r & rb->mask) == (w & rb->mask) && r == w)
        return false;

    /* Read data from the slot */
    *out = rb->slots[r & rb->mask];

    /*
     * Advance the read index. Relaxed ordering — the consumer only
     * needs to signal that the slot is available for reuse.
     */
    atomic_store_explicit(&rb->read_idx, r + 1, memory_order_relaxed);

    return true;
}

/* ------------------------------------------------------------------ */
/*  Utility                                                           */
/* ------------------------------------------------------------------ */

size_t ringbuf_size(ring_buffer_t *rb)
{
    size_t w = atomic_load_explicit(&rb->write_idx, memory_order_relaxed);
    size_t r = atomic_load_explicit(&rb->read_idx,  memory_order_relaxed);
    return w - r;
}

bool ringbuf_empty(ring_buffer_t *rb)
{
    return ringbuf_size(rb) == 0;
}

bool ringbuf_full(ring_buffer_t *rb)
{
    return ringbuf_size(rb) >= rb->capacity - 1;
}

/* ------------------------------------------------------------------ */
/*  Resize (single-threaded only)                                     */
/* ------------------------------------------------------------------ */

/*
 * Resize the ring buffer. This copies existing data into a new
 * allocation. A full memory fence ensures the new capacity, mask,
 * and slot pointer are all visible together — this guards against
 * a torn read if resize() were ever called with stale cached state.
 *
 * IMPORTANT: This is NOT safe to call concurrently with push/pop.
 * It is only used during startup calibration or reconfiguration.
 */
bool ringbuf_resize(ring_buffer_t *rb, size_t new_capacity)
{
    if (!is_power_of_two(new_capacity))
        new_capacity = next_power_of_two(new_capacity);

    if (new_capacity < ringbuf_size(rb) + 1)
        return false;  /* too small to hold current data */

    int64_t *new_slots = calloc(new_capacity, sizeof(int64_t));
    if (!new_slots)
        return false;

    /* Copy existing items into the new buffer */
    size_t r = atomic_load_explicit(&rb->read_idx, memory_order_relaxed);
    size_t w = atomic_load_explicit(&rb->write_idx, memory_order_relaxed);
    size_t count = w - r;

    for (size_t i = 0; i < count; i++) {
        new_slots[i] = rb->slots[(r + i) & rb->mask];
    }

    free(rb->slots);
    rb->slots    = new_slots;
    rb->capacity = new_capacity;
    rb->mask     = new_capacity - 1;

    /*
     * Full memory fence: ensure the capacity/mask/slots pointer
     * updates are all globally visible before we reset the indices.
     * This prevents a scenario where a stale mask is used with
     * the new (larger) slot array, which could cause out-of-bounds
     * access during a subsequent push/pop.
     */
    atomic_thread_fence(memory_order_seq_cst);

    atomic_store_explicit(&rb->read_idx,  0, memory_order_relaxed);
    atomic_store_explicit(&rb->write_idx, count, memory_order_relaxed);

    return true;
}
