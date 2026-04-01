# graphql-dataloader-per-request

## Difficulty
Extreme

## Source
Community-submitted

## Environment
Node.js 22, TypeScript, Express, express-graphql, DataLoader, better-sqlite3

## The bug
DataLoader instances are created inside each resolver function call (`userPosts` in `src/resolvers/userResolver.ts` and `postAuthor` in `src/resolvers/postResolver.ts`) instead of once per request in the context factory (`src/context.ts`). DataLoader batches all keys enqueued on the same instance within the same event-loop tick. When each resolver creates its own loader, each loader only has 1 key and batching never fires. This produces an N+1 query pattern: 100 users result in 100 individual post queries instead of 1 batched query.

## Why Extreme
1. All functional tests pass on both buggy and fixed code — the only observable difference is query count.
2. The agent must understand DataLoader's batching semantics (same-instance, same-tick grouping).
3. The fix spans 4 files: `context.ts` (add loaders), `loaders.ts` (already has correct factory — must discover it), `userResolver.ts` (remove local loader, use context), `postResolver.ts` (remove local loader, use context).
4. Red herring: `postsByDepartment` uses a JOIN query that looks like an N+1 candidate but is actually correct — it's a single query with eager loading.
5. The misleading code comments in the resolvers ("ensures isolation", "clean loader state") make the per-resolver pattern appear intentional and correct.

## Expected fix
1. Update `context.ts` to import `createLoaders` from `loaders.ts` and add the loaders to the `GqlContext` interface and `createContext()` factory.
2. Update `userResolver.ts` to use `ctx.loaders.postsByAuthorLoader.load(user.id)` in `userPosts` instead of creating a new DataLoader.
3. Update `postResolver.ts` to use `ctx.loaders.userLoader.load(post.author_id)` in `postAuthor` instead of creating a new DataLoader.
4. Remove or leave the per-resolver `makeUserLoader` / `makePostsByAuthorLoader` helper functions (they become dead code).

## Pinned at
Anonymized snapshot, original repo not disclosed
