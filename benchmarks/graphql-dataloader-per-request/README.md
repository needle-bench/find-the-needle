# graphql-dataloader-per-request

## Project

A GraphQL API server (TypeScript/Node.js) for a blogging platform with users and posts. Uses Express, `express-graphql`, DataLoader for query batching, and SQLite via `better-sqlite3`. The API supports querying users with their posts, posts with their authors, filtering posts by department, and nested queries across the user-post relationship.

## Symptoms

All functional tests pass — queries return correct data for users, posts, nested relationships, and department filtering. However, the performance test fails: fetching 100 users with their posts triggers 101+ database queries instead of the expected 2-3 (one for users, one batched query for all posts). The DataLoader batching that should collapse N individual queries into a single `WHERE id IN (...)` query is not firing.

## Bug description

DataLoader instances are created inside individual resolver functions rather than at the request level. Since DataLoader batches all keys enqueued on the **same instance** within the same event-loop tick, creating a new loader per resolver call means each loader only ever sees 1 key. Batching never fires — every resolver triggers its own individual database query, producing an N+1 pattern. The fix requires moving DataLoader instantiation from the resolvers to the GraphQL context factory so all resolvers in a single request share the same loader instances.

## Difficulty

Extreme

## Expected turns

25-40
