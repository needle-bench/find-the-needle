# ThreadSanitizer requires glibc (not musl), so we use Debian-based images.
FROM gcc:13-bookworm AS builder

RUN apt-get update && apt-get install -y --no-install-recommends \
    make \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY app/ .

# Build the optimized (non-TSan) binary to verify it compiles
RUN make clean && make

FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc \
    g++ \
    libc6-dev \
    make \
    bash \
    coreutils \
    git \
    && rm -rf /var/lib/apt/lists/* \
    && git config --global user.email "bench@needle-bench.cc" \
    && git config --global user.name "needle-bench"

WORKDIR /app
COPY app/ .

# Copy test script
COPY test.sh /app/test.sh
RUN chmod +x /app/test.sh

CMD ["/app/test.sh"]
