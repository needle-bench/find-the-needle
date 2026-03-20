#!/bin/sh
set -e
cd /workspace

# Clean any prior state
rm -f /tmp/bank.db

# Seed accounts: A=1000, B=1000
python3 app/models.py

# Start all services via supervisord
supervisord -c app/supervisord.conf &
sleep 2

# Wait for all services to be healthy
for svc_port in 8080 8081 8082; do
    for i in 1 2 3 4 5; do
        if curl -sf http://localhost:$svc_port/health > /dev/null 2>&1; then
            break
        fi
        sleep 1
    done
done

# Configure chaos: delay responses by 3 seconds (gateway timeout is 1s).
# This ensures the gateway always times out and retries, while the processor
# is still working on the first request (debit committed, idempotency not yet recorded).
curl -s -X POST http://localhost:8082/config \
    -H "Content-Type: application/json" \
    -d '{"drop_response_ms": 3000}'

# Fire several concurrent requests directly at the processor with the same
# idempotency key.  The 50ms sleep between debit-commit and idempotency-check
# creates a race window.  Sending 5 simultaneous requests makes it virtually
# certain that at least two will enter the window before the first records
# the idempotency key.
for i in 1 2 3 4 5; do
    curl -s -X POST http://localhost:8081/execute \
        -H "Content-Type: application/json" \
        -d '{"from":"A","to":"B","amount":500,"idempotency_key":"txn-001"}' > /dev/null &
done

# Wait for all curl processes to finish
wait
sleep 2

# Check balances
BALANCE_A=$(curl -s http://localhost:8080/balance/A | python3 -c "import sys,json; print(json.load(sys.stdin)['balance'])")
BALANCE_B=$(curl -s http://localhost:8080/balance/B | python3 -c "import sys,json; print(json.load(sys.stdin)['balance'])")

# Shut down services
kill $(cat /tmp/supervisord.pid) 2>/dev/null || true

echo "A=$BALANCE_A B=$BALANCE_B"
TOTAL=$((BALANCE_A + BALANCE_B))
echo "Total=$TOTAL (should be 2000)"

# Check that money is conserved: A+B should always equal 2000
# and specifically A=500, B=1500 for a single 500 transfer
if [ "$TOTAL" -ne 2000 ]; then
    echo "FAIL: money not conserved (total=$TOTAL, expected 2000) — duplicate transfer executed"
    exit 1
fi

if [ "$BALANCE_A" != "500" ] || [ "$BALANCE_B" != "1500" ]; then
    echo "FAIL: duplicate transfer (A=$BALANCE_A, B=$BALANCE_B, expected A=500 B=1500)"
    exit 1
fi
echo "PASS"
