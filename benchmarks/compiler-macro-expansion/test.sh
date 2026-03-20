#!/bin/sh
# Test for code generation correctness.
# Simple accessor tests pass, but complex nested accessors reveal
# incorrect return types in generated code.

set -e

FAIL=0

echo "=== Code Generation Engine Tests ==="

# Test 1: Simple accessors (single-level field access)
echo "--- Simple accessor test ---"
if ! codegen-engine test-simple; then
    echo "FAIL: simple accessor test"
    FAIL=1
fi

# Test 2: Complex nested accessors (chained reference access)
echo "--- Complex nested accessor test ---"
if ! codegen-engine test-complex; then
    echo "FAIL: complex nested accessor test"
    FAIL=1
fi

# Test 3: Verify generated code compiles conceptually
echo "--- Generated code inspection ---"
codegen-engine generate complex > /tmp/generated.rs
echo "Generated code written to /tmp/generated.rs"

# Check that Ref-typed fields have reference return types.
# Match "-> Company {" (owned) but not "-> &Company {" (reference).
if grep -q "\-> Company {" /tmp/generated.rs && ! grep -q "\-> &Company {" /tmp/generated.rs; then
    echo "FAIL: company getter returns owned type instead of reference"
    FAIL=1
fi
if grep -q "\-> Address {" /tmp/generated.rs && ! grep -q "\-> &Address {" /tmp/generated.rs; then
    echo "FAIL: address getter returns owned type instead of reference"
    FAIL=1
fi

rm -f /tmp/generated.rs

if [ $FAIL -eq 0 ]; then
    echo "PASS: All code generation tests passed"
    exit 0
else
    echo "FAIL: Code generation produces incorrect accessor signatures"
    exit 1
fi
