# cache-stale-invalidation

## Difficulty
Medium

## Source
Community-submitted

## Environment
Python 3, Alpine Linux

## The bug
The write endpoints (PUT and POST) in `app/server.py` update the data store but never invalidate the cache. After a product is updated or created, subsequent reads still return the cached (stale) version until the 5-minute TTL expires. The cache has a `delete()` method but the write handlers do not call it.

## Why Medium
Requires understanding the interaction between the caching layer and the write path across multiple handler methods. The agent must trace the read-cache-write flow and identify that cache invalidation calls are missing in two separate places (update and create handlers).

## Expected fix
Add `cache.delete()` calls in both the update and create handlers to invalidate the relevant cache keys (individual product and product list) after writes.

## Pinned at
Anonymized snapshot, original repo not disclosed
