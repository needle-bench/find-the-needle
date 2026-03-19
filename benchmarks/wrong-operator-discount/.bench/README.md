# wrong-operator-discount

## Difficulty
Easy

## Source
Community-submitted

## Environment
Python 3.12, Alpine Linux

## The bug
The discount calculation in `app/pricing.py` uses addition instead of multiplication: `discount = subtotal + discount_percent / 100`. This produces a nonsensical discount amount (approximately the subtotal itself for any percentage), resulting in near-zero or zero totals.

## Why Easy
Single file, single character fix. The test output shows the wildly incorrect discount amount. The arithmetic error is immediately obvious once the formula is read.

## Expected fix
Change `+` to `*` in the discount formula so it becomes `discount = subtotal * discount_percent / 100`.

## Pinned at
Anonymized snapshot, original repo not disclosed
