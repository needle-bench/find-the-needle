#!/bin/bash
# Test suite for GraphQL DataLoader benchmark.
#
# Functional tests verify correct data is returned.
# Performance test verifies that DataLoader batching reduces query count.
#
# The bug: DataLoader instances are created inside each resolver call
# instead of at the request level, so batching never fires (each loader
# has exactly 1 key).  Functional tests pass — only the performance
# assertion catches the N+1 problem.

set -e

FAIL=0
PORT=4000
BASE="http://localhost:$PORT"

echo "=== GraphQL DataLoader Per-Request Test ==="

# Recompile from source (needed after patching)
echo "--- Compiling TypeScript ---"
cd /app && npx tsc 2>&1
echo "Compile: OK"

# Re-seed database to ensure clean state
echo "--- Seeding database ---"
node /app/dist/seed.js 2>&1
echo "Seed: OK"

# Start the server in background
node /app/dist/server.js &
SERVER_PID=$!

# Wait for server to be ready
echo "--- Waiting for server ---"
for i in $(seq 1 30); do
  if curl -sf "$BASE/health" > /dev/null 2>&1; then
    break
  fi
  sleep 0.5
done

if ! curl -sf "$BASE/health" > /dev/null 2>&1; then
  echo "FAIL: Server did not start"
  kill $SERVER_PID 2>/dev/null || true
  exit 1
fi
echo "Server: OK"

# --- Functional Test 1: Fetch all users ---
echo "--- Test 1: Fetch all users ---"
RESULT=$(curl -sf -X POST "$BASE/graphql" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ users(limit: 100) { id name email department } }"}')

USER_COUNT=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d['data']['users']))")
if [ "$USER_COUNT" -ne 100 ]; then
  echo "FAIL: Expected 100 users, got $USER_COUNT"
  FAIL=1
else
  echo "OK: Got $USER_COUNT users"
fi

# --- Functional Test 2: Fetch single user with posts ---
echo "--- Test 2: Fetch single user with posts ---"
RESULT=$(curl -sf -X POST "$BASE/graphql" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ user(id: 1) { id name posts { id title author_id } } }"}')

POST_COUNT=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(len(d['data']['user']['posts']))")
if [ "$POST_COUNT" -ne 5 ]; then
  echo "FAIL: Expected 5 posts for user 1, got $POST_COUNT"
  FAIL=1
else
  echo "OK: User 1 has $POST_COUNT posts"
fi

# --- Functional Test 3: Fetch recent posts with authors ---
echo "--- Test 3: Fetch recent posts with authors ---"
RESULT=$(curl -sf -X POST "$BASE/graphql" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ recentPosts(limit: 10) { id title author { id name } } }"}')

HAS_AUTHORS=$(echo "$RESULT" | python3 -c "
import sys, json
d = json.load(sys.stdin)
posts = d['data']['recentPosts']
all_have = all(p['author'] is not None and 'name' in p['author'] for p in posts)
print('yes' if all_have else 'no')
")
if [ "$HAS_AUTHORS" != "yes" ]; then
  echo "FAIL: Not all posts have resolved authors"
  FAIL=1
else
  echo "OK: All posts have resolved authors"
fi

# --- Functional Test 4: Posts by department (JOIN query — red herring) ---
echo "--- Test 4: Posts by department (JOIN query) ---"
RESULT=$(curl -sf -X POST "$BASE/graphql" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ postsByDepartment(department: \"Engineering\", limit: 10) { id title author { id name department } } }"}')

DEPT_CHECK=$(echo "$RESULT" | python3 -c "
import sys, json
d = json.load(sys.stdin)
posts = d['data']['postsByDepartment']
all_eng = all(p['author']['department'] == 'Engineering' for p in posts)
print('yes' if all_eng and len(posts) > 0 else 'no')
")
if [ "$DEPT_CHECK" != "yes" ]; then
  echo "FAIL: postsByDepartment did not return correct results"
  FAIL=1
else
  echo "OK: postsByDepartment returns correct department-filtered posts"
fi

# --- Functional Test 5: Nested query — users with posts and post authors ---
echo "--- Test 5: Nested query correctness ---"
RESULT=$(curl -sf -X POST "$BASE/graphql" \
  -H "Content-Type: application/json" \
  -d '{"query": "{ users(limit: 5) { id name posts { id title author { id name } } } }"}')

NESTED_CHECK=$(echo "$RESULT" | python3 -c "
import sys, json
d = json.load(sys.stdin)
users = d['data']['users']
ok = True
for u in users:
    for p in u['posts']:
        if p['author']['id'] != u['id']:
            ok = False
print('yes' if ok and len(users) == 5 else 'no')
")
if [ "$NESTED_CHECK" != "yes" ]; then
  echo "FAIL: Nested query returned incorrect data"
  FAIL=1
else
  echo "OK: Nested user->posts->author references are correct"
fi

# --- Performance Test: N+1 query detection ---
echo ""
echo "=== Performance Test: Query Count ==="
echo "Fetching 100 users with their posts in a single GraphQL query."
echo "With proper DataLoader batching, this should require <=15 DB queries."
echo "(1 for users + a small number of batched post queries)"
echo ""

RESULT=$(curl -sf -X POST "$BASE/graphql" \
  -H "Content-Type: application/json" \
  -H "X-Reset-Query-Count: true" \
  -d '{"query": "{ users(limit: 100) { id name posts { id title } } }"}')

QUERY_COUNT=$(echo "$RESULT" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d['extensions']['queryCount'])")

echo "Total DB queries executed: $QUERY_COUNT"

if [ "$QUERY_COUNT" -gt 15 ]; then
  echo "FAIL: Query count $QUERY_COUNT exceeds threshold of 15"
  echo "      This indicates DataLoader batching is not working."
  echo "      Expected: 1 query for users + 1-2 batched queries for posts = ~2-3 total"
  echo "      Got: $QUERY_COUNT (likely N+1: 1 + 100 individual post queries)"
  FAIL=1
else
  echo "OK: Query count $QUERY_COUNT is within acceptable bounds (<=15)"
fi

# Cleanup
kill $SERVER_PID 2>/dev/null || true
wait $SERVER_PID 2>/dev/null || true

echo ""
if [ $FAIL -eq 0 ]; then
  echo "PASS: All tests passed"
  exit 0
else
  echo "FAIL: One or more tests failed"
  exit 1
fi
