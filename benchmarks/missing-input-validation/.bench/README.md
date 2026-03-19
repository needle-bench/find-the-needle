# missing-input-validation

## Difficulty
Easy

## Source
Community-submitted

## Environment
Node.js 20, TypeScript, Alpine Linux

## The bug
The inventory adjustment endpoint in `app/inventory.ts` and `app/routes.ts` accepts any numeric quantity without checking whether the resulting stock would go negative. A POST with `{"quantity": -200}` on an item with 100 units succeeds and sets stock to -100.

## Why Easy
Two files need changes but the pattern is straightforward. The test output clearly shows which validation is missing. The fix is a standard guard clause (check result >= 0) plus wrapping the call in a try/catch to return 400.

## Expected fix
Add a bounds check in `adjustQuantity()` that throws if the resulting quantity would be negative, and wrap the route handler in try/catch to return HTTP 400 on the error.

## Pinned at
Anonymized snapshot, original repo not disclosed
