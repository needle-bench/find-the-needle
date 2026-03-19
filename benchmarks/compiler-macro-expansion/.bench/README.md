# compiler-macro-expansion

## Difficulty
Hard

## Source
Community-submitted

## Environment
Rust 1.82, Alpine Linux

## The bug
The code generator in `src/codegen.rs` generates getters for `Ref`-typed fields with an owned return type (`-> Company`) while the method body returns a reference (`&self.company`). Simple schemas pass because Rust's auto-deref handles single-level access, but nested chained accessors (`obj.company().address()`) fail because the return type signature is wrong -- the caller expects an owned value but gets a reference.

## Why Hard
Requires understanding Rust's ownership model, the difference between owned and borrowed return types in generated code, and why auto-deref masks the bug for simple cases. The agent must read the code generation logic, understand the schema type system (Primitive, Optional, List, Ref), and identify the specific format string that produces the incorrect signature. The simple test suite passes, providing a false sense of correctness.

## Expected fix
Add `&` before the type name in the Ref variant's format string so the generated getter returns `&Company` instead of `Company`.

## Pinned at
Anonymized snapshot, original repo not disclosed
