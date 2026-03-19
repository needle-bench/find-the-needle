# type-coercion-comparison

## Difficulty
Easy

## Source
Community-submitted

## Environment
Node.js 20, Alpine Linux

## The bug
The rating filter in `app/utils.js` reads the `min_rating` query parameter as a string (via `searchParams.get()`) but compares it against numeric rating values in the product data. JavaScript string-to-number comparison (`"4" >= 4`) produces inconsistent results, causing the filter to return wrong product counts.

## Why Easy
Single file, single line fix. The test output shows exactly how many products are returned vs. expected. The query parameter parsing is isolated in one utility function, and the fix is a standard `parseInt()` call.

## Expected fix
Parse the `min_rating` query parameter with `parseInt(..., 10)` before storing it, so the comparison uses numeric values on both sides.

## Pinned at
Anonymized snapshot, original repo not disclosed
