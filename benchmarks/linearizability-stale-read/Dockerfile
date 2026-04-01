FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY app/ .

RUN go build -o kv-cluster .

FROM alpine:3.20

RUN apk add --no-cache bash coreutils git && \
    git config --global user.email "bench@needle-bench.cc" && \
    git config --global user.name "needle-bench"

COPY --from=builder /build/kv-cluster /usr/local/bin/kv-cluster

WORKDIR /app

COPY . .
RUN chmod +x test.sh && \
    git init && git add -A && git commit -m 'init'

# Install Go toolchain in final image so agent can rebuild
RUN apk add --no-cache go && \
    rm -rf /var/cache/apk/*

CMD ["kv-cluster"]
