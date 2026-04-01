FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY app/ .

RUN go build -o tlsserver .

FROM alpine:3.20

RUN apk add --no-cache bash coreutils git openssl curl && \
    git config --global user.email "bench@needle-bench.cc" && \
    git config --global user.name "needle-bench"

COPY --from=builder /build/tlsserver /usr/local/bin/tlsserver

# Install Go toolchain in final image so agent can rebuild
RUN apk add --no-cache go && \
    rm -rf /var/cache/apk/*

# Generate a 3-level certificate chain: Root CA -> Intermediate CA -> Leaf
# Certificates are written to /certs/
RUN mkdir -p /certs && \
    # Root CA \
    openssl genrsa -out /certs/root.key 2048 && \
    openssl req -x509 -new -nodes -key /certs/root.key \
        -sha256 -days 3650 \
        -subj "/C=US/ST=Test/L=Test/O=TestRootCA/CN=Test Root CA" \
        -out /certs/root.crt && \
    # Intermediate CA \
    openssl genrsa -out /certs/intermediate.key 2048 && \
    openssl req -new -nodes -key /certs/intermediate.key \
        -subj "/C=US/ST=Test/L=Test/O=TestIntermediateCA/CN=Test Intermediate CA" \
        -out /certs/intermediate.csr && \
    printf "basicConstraints=CA:TRUE\nkeyUsage=keyCertSign,cRLSign\n" > /certs/intermediate.ext && \
    openssl x509 -req -in /certs/intermediate.csr \
        -CA /certs/root.crt -CAkey /certs/root.key -CAcreateserial \
        -sha256 -days 1825 \
        -extfile /certs/intermediate.ext \
        -out /certs/intermediate.crt && \
    # Leaf (server) cert \
    openssl genrsa -out /certs/leaf.key 2048 && \
    openssl req -new -nodes -key /certs/leaf.key \
        -subj "/C=US/ST=Test/L=Test/O=TestServer/CN=localhost" \
        -out /certs/leaf.csr && \
    printf "subjectAltName=DNS:localhost,IP:127.0.0.1\nbasicConstraints=CA:FALSE\nkeyUsage=digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n" > /certs/leaf.ext && \
    openssl x509 -req -in /certs/leaf.csr \
        -CA /certs/intermediate.crt -CAkey /certs/intermediate.key -CAcreateserial \
        -sha256 -days 365 \
        -extfile /certs/leaf.ext \
        -out /certs/leaf.crt && \
    # Assemble full chain PEM for the server (all 3 certs) \
    # BUG: chain order is leaf, root, intermediate instead of \
    #      leaf, intermediate, root (RFC 5246 Section 7.4.2). \
    #      Lenient clients tolerate this; strict validators reject it. \
    cat /certs/leaf.crt /certs/root.crt /certs/intermediate.crt > /certs/chain.pem && \
    # Clean up temporary files \
    rm -f /certs/*.csr /certs/*.srl /certs/*.ext

WORKDIR /app

COPY . .
RUN chmod +x test.sh

CMD ["tlsserver", "serve"]
