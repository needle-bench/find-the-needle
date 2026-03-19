# import-cycle-startup

## Difficulty
Easy

## Source
Community-submitted

## Environment
Python 3.12, Alpine Linux

## The bug
Modules `app/users.py` and `app/notifications.py` form a circular import. `notifications.py` imports the `users` module at the top level to access `users.DEFAULT_ROLE`, but `users.py` also imports from `notifications`. On cold start, Python partially initializes `users` before `notifications` finishes loading, causing an `AttributeError` because `DEFAULT_ROLE` is not yet defined when `notifications` tries to access it.

## Why Easy
The crash traceback points directly at the circular import. The fix is a standard Python pattern: move the shared constant to `config.py` to break the cycle. Requires changes across three files but each change is trivial.

## Expected fix
Move `DEFAULT_ROLE` to `config.py`, import it from there in both `users.py` and `notifications.py`, eliminating the circular dependency.

## Pinned at
Anonymized snapshot, original repo not disclosed
