import DataLoader from "dataloader";
import Database from "better-sqlite3";
import { trackedQuery } from "./db";

/**
 * Batch-load users by their IDs.
 *
 * DataLoader will collect all user-id requests made in the same tick
 * and call this function once with all the IDs.  This turns N individual
 * SELECT queries into a single SELECT ... WHERE id IN (...) query.
 */
function batchUsers(db: Database.Database) {
  return async (ids: readonly number[]): Promise<any[]> => {
    const placeholders = ids.map(() => "?").join(",");
    const rows = trackedQuery(
      db,
      `SELECT id, name, email, department FROM users WHERE id IN (${placeholders})`,
      ...ids
    );
    const byId = new Map(rows.map((r: any) => [r.id, r]));
    return ids.map((id) => byId.get(id) || null);
  };
}

/**
 * Batch-load posts by their author IDs.
 *
 * Given a list of user IDs, returns posts grouped so that each user-id
 * maps to an array of posts.  Turns N per-user queries into one.
 */
function batchPostsByAuthor(db: Database.Database) {
  return async (authorIds: readonly number[]): Promise<any[][]> => {
    const placeholders = authorIds.map(() => "?").join(",");
    const rows = trackedQuery(
      db,
      `SELECT id, title, body, author_id, created_at FROM posts WHERE author_id IN (${placeholders})`,
      ...authorIds
    );
    const grouped = new Map<number, any[]>();
    for (const row of rows) {
      const list = grouped.get(row.author_id) || [];
      list.push(row);
      grouped.set(row.author_id, list);
    }
    return authorIds.map((id) => grouped.get(id) || []);
  };
}

/**
 * Create all DataLoader instances for a single request.
 *
 * IMPORTANT: DataLoader batching relies on all keys being enqueued on
 * the same loader instance within the same tick.  Loaders MUST be
 * created once per request (not once per resolver call, not globally).
 */
export function createLoaders(db: Database.Database) {
  return {
    userLoader: new DataLoader(batchUsers(db)),
    postsByAuthorLoader: new DataLoader(batchPostsByAuthor(db)),
  };
}
