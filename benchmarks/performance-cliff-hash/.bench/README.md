# performance-cliff-hash

## Difficulty
Hard

## Source
Community-submitted

## Environment
Java 17, Alpine Linux

## The bug
The `hashCode()` method in `src/Product.java` hashes on the `category` field instead of the unique `sku` field. Since many products share the same category, this causes massive hash collisions. At small scale the chains are short enough to appear performant, but at 10,000 entries the O(n) chain traversal dominates lookup time. The `getBucketIndexForSku()` method in `ProductCache.java` also needs updating since it reconstructs products to compute hash codes.

## Why Hard
Requires understanding hash map internals, collision resolution via separate chaining, and performance analysis. The small-dataset test passes, creating a false sense of correctness. The agent must connect the performance degradation to the hash distribution, identify that `hashCode()` uses the wrong field, and also fix the dependent lookup path in `ProductCache`.

## Expected fix
Change `hashCode()` to hash on `sku` instead of `category`, and update `getBucketIndexForSku()` to hash the SKU string directly instead of reconstructing a product.

## Pinned at
Anonymized snapshot, original repo not disclosed
