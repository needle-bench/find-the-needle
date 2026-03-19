# sql-injection-search

## Difficulty
Medium

## Source
Community-submitted

## Environment
Python 3, SQLite, Alpine Linux

## The bug
The search function in `app/search.py` concatenates user input directly into a SQL query string using f-strings: `f"...WHERE name LIKE '%{query}%'"`. This allows SQL injection via UNION-based attacks (extracting data from the users table) and boolean-based attacks (bypassing the WHERE clause with `' OR '1'='1`).

## Why Medium
Requires understanding SQL injection mechanics and the difference between raw string interpolation and parameterized queries. The database layer has both safe (`execute`) and unsafe (`execute_raw`) methods, so the agent must identify which is being used and switch to the correct one. The fix also needs to handle the optional category filter.

## Expected fix
Replace the f-string SQL construction with parameterized queries using `?` placeholders, and switch from `execute_raw()` to `execute()` with a params tuple.

## Pinned at
Anonymized snapshot, original repo not disclosed
