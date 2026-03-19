# off-by-one-array-slice

## Difficulty
Easy

## Source
Community-submitted

## Environment
Python 3.12, Alpine Linux

## The bug
The batch slicing in `app/processor.py` uses `end = start + batch_size - 1` to compute the slice endpoint. Since Python slices are exclusive on the upper bound, subtracting 1 causes each batch to be one element short. Records at each batch boundary are silently dropped.

## Why Easy
Single file, single line fix. The test output directly reports the missing record count. The slicing logic is isolated in one function and the off-by-one pattern is immediately recognizable.

## Expected fix
Remove the `- 1` from the slice end calculation so it becomes `end = start + batch_size`.

## Pinned at
Anonymized snapshot, original repo not disclosed
