import DataLoader from "dataloader";
import { GqlContext } from "../context";
import { trackedQuery } from "../db";

/**
 * Create a fresh DataLoader to batch-fetch users by ID.
 *
 * Each call produces a new loader, ensuring isolation between
 * different parts of the query tree.
 */
function makeUserLoader(ctx: GqlContext) {
  return new DataLoader(async (ids: readonly number[]) => {
    const placeholders = ids.map(() => "?").join(",");
    const rows = trackedQuery(
      ctx.db,
      `SELECT id, name, email, department FROM users WHERE id IN (${placeholders})`,
      ...ids
    );
    const byId = new Map(rows.map((r: any) => [r.id, r]));
    return ids.map((id) => byId.get(id) || null);
  });
}

export const userResolvers = {
  /**
   * Top-level query: fetch all users.
   */
  users(_: any, args: any, ctx: GqlContext) {
    const limit = args.limit || 100;
    return trackedQuery(ctx.db, "SELECT id, name, email, department FROM users LIMIT ?", limit);
  },

  /**
   * Top-level query: fetch a single user by ID.
   */
  user(_: any, args: { id: number }, ctx: GqlContext) {
    const loader = makeUserLoader(ctx);
    return loader.load(args.id);
  },

  /**
   * Field resolver: User.posts — fetch posts authored by this user.
   *
   * Creates a new DataLoader for each User object being resolved.
   * This guarantees each resolver invocation gets a clean loader.
   */
  userPosts(user: any, _args: any, ctx: GqlContext) {
    const loader = new DataLoader(async (authorIds: readonly number[]) => {
      const placeholders = authorIds.map(() => "?").join(",");
      const rows = trackedQuery(
        ctx.db,
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
    });

    return loader.load(user.id);
  },
};
