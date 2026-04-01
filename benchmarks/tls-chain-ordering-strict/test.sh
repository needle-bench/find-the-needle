#!/bin/sh
# Test TLS certificate chain ordering.
#
# RFC 5246 Section 7.4.2 requires the server to send certificates in order:
#   cert[0] = leaf (server cert)
#   cert[1] = issuer of cert[0] (intermediate CA)
#   cert[2] = issuer of cert[1] (root CA, optional)
#
# Lenient TLS clients (curl, Go, browsers) build a cert graph and can
# handle any order. Strict validators process the chain sequentially and
# reject out-of-order chains.

set -e

FAIL=0

echo "=== TLS Chain Ordering Test ==="

# Rebuild from source (needed after patching)
echo "--- Building ---"
cd /app/app && go build -o /usr/local/bin/tlsserver . 2>&1

# Start the server in the background
echo "--- Starting HTTPS server ---"
cd /app
tlsserver serve &
SERVER_PID=$!

# Wait for server to be ready
sleep 2

# Verify server is running
if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "FAIL: Server failed to start"
    exit 1
fi

echo "--- Test 1: Lenient TLS client ---"
# curl with --cacert is lenient — it builds a trust graph from all certs
# regardless of order. This should always succeed.
RESPONSE=$(curl --cacert /certs/root.crt \
    --connect-timeout 5 \
    -s -o /dev/null -w "%{http_code}" \
    https://localhost:8443/health 2>/dev/null || true)

if [ "$RESPONSE" = "200" ]; then
    echo "OK: Lenient client connected successfully"
else
    echo "FAIL: Lenient client could not connect (HTTP $RESPONSE)"
    FAIL=1
fi

echo "--- Test 2: Chain order verification (strict RFC 5246) ---"
# Extract each certificate from the server's chain using openssl s_client
# -showcerts, then verify that each cert[i].Issuer == cert[i+1].Subject.

# Dump the full chain from the server
CERT_DIR=$(mktemp -d)

echo "Q" | openssl s_client \
    -connect localhost:8443 \
    -showcerts \
    -CAfile /certs/root.crt \
    2>/dev/null > "$CERT_DIR/raw.txt" || true

# Split individual PEM certificates into separate files
awk '/-----BEGIN CERTIFICATE-----/{n++} n{print > "'"$CERT_DIR"'/cert-" n ".pem"}' "$CERT_DIR/raw.txt"

NUM_CERTS=$(ls "$CERT_DIR"/cert-*.pem 2>/dev/null | wc -l | tr -d ' ')

if [ "$NUM_CERTS" -lt 2 ]; then
    echo "FAIL: Expected at least 2 certificates in chain, got $NUM_CERTS"
    FAIL=1
else
    # Extract subject and issuer DN hashes for each cert.
    # Using -subject_hash / -issuer_hash gives portable 8-hex-char
    # hashes that are consistent regardless of DN formatting options.
    i=1
    while [ $i -le "$NUM_CERTS" ]; do
        openssl x509 -in "$CERT_DIR/cert-$i.pem" -noout -subject_hash 2>/dev/null > "$CERT_DIR/subhash-$i.txt"
        openssl x509 -in "$CERT_DIR/cert-$i.pem" -noout -issuer_hash 2>/dev/null > "$CERT_DIR/isshash-$i.txt"
        openssl x509 -in "$CERT_DIR/cert-$i.pem" -noout -subject 2>/dev/null > "$CERT_DIR/subject-$i.txt"
        openssl x509 -in "$CERT_DIR/cert-$i.pem" -noout -issuer 2>/dev/null > "$CERT_DIR/issuer-$i.txt"
        i=$((i + 1))
    done

    # Check: cert[i].Issuer should match cert[i+1].Subject
    # Compare using DN hashes for reliability.
    CHAIN_OK=1
    i=1
    while [ $i -lt "$NUM_CERTS" ]; do
        CURR_ISSUER_HASH=$(cat "$CERT_DIR/isshash-$i.txt" | xargs)
        NEXT_SUBJECT_HASH=$(cat "$CERT_DIR/subhash-$((i+1)).txt" | xargs)

        if [ "$CURR_ISSUER_HASH" != "$NEXT_SUBJECT_HASH" ]; then
            CURR_ISSUER=$(cat "$CERT_DIR/issuer-$i.txt")
            NEXT_SUBJECT=$(cat "$CERT_DIR/subject-$((i+1)).txt")
            echo "FAIL: Chain order violation at position $((i-1)):"
            echo "      cert[$((i-1))].Issuer  = $CURR_ISSUER"
            echo "      cert[$i].Subject = $NEXT_SUBJECT"
            echo "      Expected these to match (cert[$i] should be the issuer of cert[$((i-1))])"
            CHAIN_OK=0
        fi
        i=$((i + 1))
    done

    if [ "$CHAIN_OK" = "1" ]; then
        echo "OK: Chain is in correct RFC 5246 order"
    else
        FAIL=1
    fi
fi

echo "--- Test 3: Leaf certificate identity ---"
# Verify the first certificate in the chain is the leaf (CN=localhost).
if [ -f "$CERT_DIR/cert-1.pem" ]; then
    FIRST_SUBJECT=$(openssl x509 -in "$CERT_DIR/cert-1.pem" -noout -subject 2>/dev/null)

    if echo "$FIRST_SUBJECT" | grep -q "CN.*=.*localhost"; then
        echo "OK: First certificate is the leaf (CN=localhost)"
    else
        echo "FAIL: First certificate is NOT the leaf"
        echo "      Got: $FIRST_SUBJECT"
        FAIL=1
    fi
else
    echo "FAIL: Could not extract first certificate from chain"
    FAIL=1
fi

# Cleanup
rm -rf "$CERT_DIR"
kill "$SERVER_PID" 2>/dev/null || true
wait "$SERVER_PID" 2>/dev/null || true

echo ""
if [ $FAIL -eq 0 ]; then
    echo "PASS: TLS chain is correctly ordered per RFC 5246"
    exit 0
else
    echo "FAIL: TLS chain ordering issues detected"
    exit 1
fi
