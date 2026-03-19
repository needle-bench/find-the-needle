# off-by-one-pagination

## Difficulty
Easy

## Source
Community-submitted

## Environment
Python 3.12, Flask, Alpine Linux

## The bug
The pagination offset formula in `app/app.py` subtracts 1 for pages after the first: `offset = (page - 1) * per_page - (1 if page > 1 else 0)`. This causes each page after page 1 to start one item too early, skipping items at page boundaries.

## Why Easy
Single file, single line fix. The test output clearly shows the page boundary error with exact IDs. The offset formula is the only pagination logic in the codebase.

## Expected fix
Remove the erroneous `- (1 if page > 1 else 0)` from the offset calculation so it becomes `offset = (page - 1) * per_page`.

## Pinned at
Anonymized snapshot, original repo not disclosed
