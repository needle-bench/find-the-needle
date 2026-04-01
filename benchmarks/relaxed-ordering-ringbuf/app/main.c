/*
 * main.c — SPSC ring buffer stress test.
 *
 * Producer thread pushes N sequential integers (0..N-1) into the ring
 * buffer; consumer thread pops them and verifies strict sequential
 * ordering plus data integrity. On a correct implementation, all N
 * items arrive in order with no gaps and no corruption.
 *
 * Compile with:
 *   gcc -std=c11 -pthread -O2 -o ringbuf_test ring_buffer.c main.c
 *
 * For ThreadSanitizer:
 *   gcc -std=c11 -pthread -fsanitize=thread -O1 -g -o ringbuf_test ring_buffer.c main.c
 */

#include "ring_buffer.h"

#include <stdio.h>
#include <stdlib.h>
#include <pthread.h>
#include <inttypes.h>
#include <string.h>

#define NUM_ITEMS       1048576   /* 1M items */
#define RINGBUF_SIZE    8192      /* power of 2 */

typedef struct {
    ring_buffer_t *rb;
    int64_t        num_items;
} thread_arg_t;

/* ------------------------------------------------------------------ */
/*  Producer thread                                                   */
/* ------------------------------------------------------------------ */

static void *producer_thread(void *arg)
{
    thread_arg_t *ta = (thread_arg_t *)arg;
    ring_buffer_t *rb = ta->rb;
    int64_t n = ta->num_items;

    for (int64_t i = 0; i < n; i++) {
        /* Spin until there's space. In a real system you'd use
         * a futex or condition variable, but for a stress test
         * busy-wait is fine. */
        while (!ringbuf_push(rb, i)) {
            /* spin */
        }
    }

    return NULL;
}

/* ------------------------------------------------------------------ */
/*  Consumer thread                                                   */
/* ------------------------------------------------------------------ */

typedef struct {
    int64_t items_received;
    int64_t first_error_idx;
    int64_t first_error_got;
    bool    has_error;
} consumer_result_t;

static void *consumer_thread(void *arg)
{
    thread_arg_t *ta = (thread_arg_t *)arg;
    ring_buffer_t *rb = ta->rb;
    int64_t n = ta->num_items;

    consumer_result_t *result = calloc(1, sizeof(consumer_result_t));
    result->first_error_idx = -1;

    int64_t expected = 0;
    int64_t received = 0;

    while (received < n) {
        int64_t value;
        if (ringbuf_pop(rb, &value)) {
            if (value != expected && !result->has_error) {
                result->has_error = true;
                result->first_error_idx = expected;
                result->first_error_got = value;
            }
            expected++;
            received++;
        }
        /* else: spin */
    }

    result->items_received = received;
    return result;
}

/* ------------------------------------------------------------------ */
/*  Main — run test and report                                        */
/* ------------------------------------------------------------------ */

int main(int argc, char *argv[])
{
    int64_t num_items = NUM_ITEMS;

    /* Optional: allow overriding item count from command line */
    if (argc > 1) {
        num_items = atoll(argv[1]);
        if (num_items <= 0) {
            fprintf(stderr, "Usage: %s [num_items]\n", argv[0]);
            return 2;
        }
    }

    printf("=== SPSC Ring Buffer Test ===\n");
    printf("Items:    %" PRId64 "\n", num_items);
    printf("Capacity: %d\n", RINGBUF_SIZE);
    printf("\n");

    /* Create ring buffer */
    ring_buffer_t *rb = ringbuf_create(RINGBUF_SIZE);
    if (!rb) {
        fprintf(stderr, "Failed to allocate ring buffer\n");
        return 2;
    }

    /* Optionally exercise resize path (calibration) */
    if (!ringbuf_resize(rb, RINGBUF_SIZE)) {
        fprintf(stderr, "Warning: resize calibration failed\n");
    }

    /* Launch threads */
    thread_arg_t ta = { .rb = rb, .num_items = num_items };

    pthread_t prod, cons;
    if (pthread_create(&prod, NULL, producer_thread, &ta) != 0) {
        fprintf(stderr, "Failed to create producer thread\n");
        return 2;
    }
    if (pthread_create(&cons, NULL, consumer_thread, &ta) != 0) {
        fprintf(stderr, "Failed to create consumer thread\n");
        return 2;
    }

    pthread_join(prod, NULL);

    consumer_result_t *result;
    pthread_join(cons, (void **)&result);

    /* Report */
    printf("Producer: sent %" PRId64 " items\n", num_items);
    printf("Consumer: received %" PRId64 " items\n", result->items_received);

    int exit_code = 0;

    if (result->has_error) {
        printf("FAIL: Data integrity error at index %" PRId64
               " — expected %" PRId64 ", got %" PRId64 "\n",
               result->first_error_idx,
               result->first_error_idx,
               result->first_error_got);
        exit_code = 1;
    } else if (result->items_received != num_items) {
        printf("FAIL: Item count mismatch — expected %" PRId64
               ", received %" PRId64 "\n",
               num_items, result->items_received);
        exit_code = 1;
    } else {
        printf("OK: All %" PRId64 " items transferred correctly\n", num_items);
    }

    free(result);
    ringbuf_destroy(rb);

    return exit_code;
}
