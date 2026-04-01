CC      = gcc
CFLAGS  = -std=c11 -pthread -Wall -Wextra -Wpedantic

# Default: optimized build (production-like)
OPTFLAGS = -O2

# ThreadSanitizer build: lower optimization for better diagnostics
TSAN_FLAGS = -fsanitize=thread -O1 -g

SRCS = ring_buffer.c main.c
TARGET = ringbuf_test
TARGET_TSAN = ringbuf_test_tsan

.PHONY: all clean tsan

all: $(TARGET)

$(TARGET): $(SRCS) ring_buffer.h
	$(CC) $(CFLAGS) $(OPTFLAGS) -o $@ $(SRCS)

tsan: $(TARGET_TSAN)

$(TARGET_TSAN): $(SRCS) ring_buffer.h
	$(CC) $(CFLAGS) $(TSAN_FLAGS) -o $@ $(SRCS)

clean:
	rm -f $(TARGET) $(TARGET_TSAN)
